package gen

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Shaitrish/37studio/internal/costs"
	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/remote"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

// Brand voices are ElevenLabs voices saved per brand, cloned here from a recording or picked from the account, so
// every video for the brand can speak in them by name. The voice itself lives at ElevenLabs and is made once; the row
// keeps its id, and the brand folder keeps the recording so the editor can hear it (or clone it again on another key).

func voiceDir(tenant int64, voiceID string) string {
	return filepath.Join(files.TenantRoot(tenant), "voices", files.SafeName(voiceID))
}

// VoiceFor picks the voice for a brand's video in a language: the brand's default for that language, then its default
// for any language, then the app-wide one from Settings (Bangla has its own).
func VoiceFor(tenant int64, lang string) string {
	var id string
	store.DB.QueryRow(`select voice_id from brand_voices where tenant_id=? and is_default=1 and language in (?, '')
	  order by language=? desc limit 1`, tenant, lang, lang).Scan(&id)
	if id != "" {
		return id
	}
	if lang == "bn" && store.Setting("voice_bn") != "" {
		return store.Setting("voice_bn")
	}
	return store.Setting("voice_default")
}

var elevenID = regexp.MustCompile(`^[A-Za-z0-9]{20}$`)

// ResolveVoice turns what the agent or editor asked for into an ElevenLabs voice id: one of the brand's voices by name
// (any case), or an ElevenLabs id as is. Empty means the brand's default for the language.
func ResolveVoice(tenant int64, lang, want string) (string, error) {
	want = strings.TrimSpace(want)
	if want == "" {
		return VoiceFor(tenant, lang), nil
	}
	var id string
	store.DB.QueryRow("select voice_id from brand_voices where tenant_id=? and (lower(name)=lower(?) or voice_id=?) order by is_default desc limit 1",
		tenant, want, want).Scan(&id)
	if id != "" {
		return id, nil
	}
	if elevenID.MatchString(want) {
		return want, nil
	}
	var names []string
	rows, _ := store.Rows("select name from brand_voices where tenant_id=? order by name", tenant)
	for _, r := range rows {
		names = append(names, fmt.Sprintf("%q", r["name"]))
	}
	if len(names) == 0 {
		return "", fmt.Errorf("this brand has no voice called %q, and no saved voices yet: add one on the brand's Voices tab", want)
	}
	return "", fmt.Errorf("this brand has no voice called %q; its voices are %s", want, strings.Join(names, ", "))
}

func ListBrandVoices(r *http.Request) (any, error) {
	tenant := httpapi.PathID(r)
	vs, err := store.Rows("select * from brand_voices where tenant_id=? order by is_default desc, name", tenant)
	for _, v := range vs {
		v["samples"] = files.List(voiceDir(tenant, store.Str(v["voice_id"])), fmt.Sprintf("/files/tenants/%d/voices/%s", tenant, files.SafeName(store.Str(v["voice_id"]))), false)
	}
	return vs, err
}

// AddBrandVoice saves a voice already in the ElevenLabs account (premade, from their library, or cloned there) to a brand.
func AddBrandVoice(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	tenant, voiceID := httpapi.PathID(r), strings.TrimSpace(store.Str(b["voice_id"]))
	if voiceID == "" {
		return nil, httpapi.Bad("pick an ElevenLabs voice")
	}
	raw, err := eleven("GET", "/v1/voices/"+voiceID, nil)
	if err != nil {
		return nil, httpapi.Bad("ElevenLabs doesn't know that voice: " + err.Error())
	}
	var v struct {
		Name string `json:"name"`
	}
	json.Unmarshal(raw, &v)
	b["name"] = cmp.Or(strings.TrimSpace(store.Str(b["name"])), v.Name)
	return saveBrandVoice(tenant, voiceID, false, b)
}

// CloneBrandVoice makes a new ElevenLabs voice from recordings of one person (instant voice cloning) and saves it to
// the brand. The recordings come as a multipart upload; name, language, notes, default and remove_noise ride in the
// query, so files.SaveUploads can stream the files to disk.
func CloneBrandVoice(r *http.Request) (any, error) {
	tenant, q := httpapi.PathID(r), r.URL.Query()
	name := strings.TrimSpace(q.Get("name"))
	if name == "" {
		return nil, httpapi.Bad("give the voice a name")
	}
	os.MkdirAll(files.TenantRoot(tenant), 0o755)
	tmp, err := os.MkdirTemp(files.TenantRoot(tenant), ".voice-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	saved, err := files.SaveUploads(r, filepath.Join(tmp, "in"))
	if err != nil {
		return nil, err
	}
	names, _ := saved.([]string)
	if len(names) == 0 {
		return nil, httpapi.Bad("add a recording of the voice: a minute or two of one person speaking clearly works best")
	}
	var samples []string
	for i, n := range names {
		out := filepath.Join(tmp, fmt.Sprintf("sample-%d.mp3", i+1))
		// Audio only, mono, at most 5 minutes: a video works as a sample too, and cloning gains nothing from more.
		cmd := exec.Command("ffmpeg", "-y", "-v", "error", "-i", filepath.Join(tmp, "in", n), "-vn", "-ac", "1", "-ar", "44100", "-b:a", "128k", "-t", "300", out)
		if msg, err := cmd.CombinedOutput(); err != nil {
			return nil, httpapi.Bad(fmt.Sprintf("couldn't read the audio in %s: %s", n, textutil.Tail(strings.TrimSpace(string(msg)), 200)))
		}
		samples = append(samples, out)
	}
	os.RemoveAll(filepath.Join(tmp, "in"))
	voiceID, err := cloneVoice(name, q.Get("notes"), q.Get("remove_noise") == "1", samples)
	if err != nil {
		return nil, httpapi.Bad(err.Error())
	}
	dir := voiceDir(tenant, voiceID)
	os.MkdirAll(filepath.Dir(dir), 0o755)
	if err := os.Rename(tmp, dir); err != nil {
		return nil, err
	}
	return saveBrandVoice(tenant, voiceID, true, map[string]any{"name": name, "language": q.Get("language"), "notes": q.Get("notes"), "is_default": q.Get("default") == "1"})
}

// cloneVoice uploads the samples to ElevenLabs. They are at most 5 minutes of mono MP3 each, a few MB, so the form is
// built in memory.
func cloneVoice(name, notes string, removeNoise bool, samples []string) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", name)
	if notes = strings.TrimSpace(notes); notes != "" {
		mw.WriteField("description", textutil.Head(notes, 500))
	}
	mw.WriteField("remove_background_noise", strconv.FormatBool(removeNoise))
	for _, s := range samples {
		part, _ := mw.CreateFormFile("files", filepath.Base(s))
		f, err := os.Open(s)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(part, f)
		f.Close()
		if err != nil {
			return "", err
		}
	}
	mw.Close()
	req, _ := http.NewRequest("POST", elevenURL+"/v1/voices/add", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	raw, err := elevenSend(req)
	if err != nil {
		return "", err
	}
	var res struct {
		ID string `json:"voice_id"`
	}
	if json.Unmarshal(raw, &res); res.ID == "" {
		return "", fmt.Errorf("ElevenLabs didn't return a voice: %s", textutil.Tail(string(raw), 200))
	}
	return res.ID, nil
}

var voiceCols = []string{"name", "language", "notes"}

func saveBrandVoice(tenant int64, voiceID string, cloned bool, b map[string]any) (any, error) {
	if strings.TrimSpace(store.Str(b["name"])) == "" {
		return nil, httpapi.Bad("give the voice a name")
	}
	res, err := store.DB.Exec("insert into brand_voices(tenant_id, voice_id, cloned, name) values(?,?,?,?)", tenant, voiceID, flag(cloned), b["name"])
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return updateBrandVoice(id, b)
}

func flag(b bool) int {
	if b {
		return 1
	}
	return 0
}

func UpdateBrandVoice(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	if _, err := store.Row("select id from brand_voices where id=?", httpapi.PathID(r)); err != nil {
		return nil, err
	}
	return updateBrandVoice(httpapi.PathID(r), b)
}

// updateBrandVoice saves the editable fields; is_default makes it the brand's one default for its language.
func updateBrandVoice(id int64, b map[string]any) (any, error) {
	if n, ok := b["name"]; ok && strings.TrimSpace(store.Str(n)) == "" {
		return nil, httpapi.Bad("give the voice a name")
	}
	if _, err := store.Save("brand_voices", voiceCols, id, b); err != nil {
		return nil, err
	}
	if d, ok := b["is_default"]; ok {
		store.DB.Exec("update brand_voices set is_default=? where id=?", flag(d == true || store.Num(d) == 1), id)
	}
	// One default per brand and language; the newest pick wins.
	if _, err := store.DB.Exec(`update brand_voices set is_default=0 where id<>? and is_default=1
	  and (tenant_id, language) = (select tenant_id, language from brand_voices where id=? and is_default=1)`, id, id); err != nil {
		return nil, err
	}
	return store.Row("select * from brand_voices where id=?", id)
}

func DeleteBrandVoice(r *http.Request) (any, error) {
	v, err := store.Row("select * from brand_voices where id=?", httpapi.PathID(r))
	if err != nil {
		return nil, err
	}
	if _, err := store.DB.Exec("delete from brand_voices where id=?", v["id"]); err != nil {
		return nil, err
	}
	tenant, voiceID := v["tenant_id"].(int64), store.Str(v["voice_id"])
	var others int
	store.DB.QueryRow("select count(*) from brand_voices where voice_id=? and tenant_id=?", voiceID, tenant).Scan(&others)
	if others == 0 {
		os.RemoveAll(voiceDir(tenant, voiceID))
	}
	if v["cloned"] == int64(1) {
		DeleteVoices([]string{voiceID})
	}
	return nil, nil
}

// ClonedVoices are the voices a brand made here, to delete at ElevenLabs when the brand goes.
func ClonedVoices(tenant int64) []string {
	rows, _ := store.Rows("select voice_id from brand_voices where tenant_id=? and cloned=1", tenant)
	var ids []string
	for _, r := range rows {
		ids = append(ids, store.Str(r["voice_id"]))
	}
	return ids
}

// DeleteVoices removes cloned voices from the ElevenLabs account unless another brand still uses them. Best effort:
// a removed key or a voice already deleted there shouldn't stop the editor from tidying up here.
func DeleteVoices(ids []string) {
	for _, id := range ids {
		var n int
		store.DB.QueryRow("select count(*) from brand_voices where voice_id=?", id).Scan(&n)
		if n == 0 {
			eleven("DELETE", "/v1/voices/"+id, nil)
		}
	}
}

// Revoice speaks an existing recording again in another voice, keeping its words, timing and emotion (ElevenLabs voice
// changer), so captions and animation timed to the old voiceover still line up.
func Revoice(pid, tenant int64, in, voice, out string) error {
	if voice == "" {
		return fmt.Errorf("no voice to change to: name one of the brand's voices")
	}
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	sec := files.MediaSeconds(in)
	req := remote.Multipart(elevenURL+"/v1/voice-changer/"+voice+"?output_format=mp3_44100_128",
		map[string]string{"model_id": store.Setting("revoice_model")}, "audio", filepath.Base(in), f, "")
	audio, err := elevenSend(req)
	if err != nil {
		return err
	}
	// Through a temp file: out is often the file being replaced.
	if err := os.WriteFile(out+".part", audio, 0o644); err != nil {
		return err
	}
	if err := os.Rename(out+".part", out); err != nil {
		return err
	}
	costs.Record("elevenlabs", pid, tenant, "voice change "+store.Setting("revoice_model"), sec, sec/60*store.SettingF("revoice_price_per_min"), out)
	return nil
}
