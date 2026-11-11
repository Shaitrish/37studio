package agent

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Shaitrish/37studio/internal/files"
	"github.com/Shaitrish/37studio/internal/httpapi"
	"github.com/Shaitrish/37studio/internal/proc"
	"github.com/Shaitrish/37studio/internal/store"
	"github.com/Shaitrish/37studio/internal/textutil"
)

// Frame sizes a copy can be made in, in pixels: what each platform's feed shows best.
var frameSizes = map[string][2]int{"9:16": {1080, 1920}, "16:9": {1920, 1080}, "1:1": {1080, 1080}, "4:5": {1080, 1350}}

// resizeFilter is the ffmpeg filter that puts a video into a w×h frame. "crop" fills the frame and cuts the edges off;
// "fit" keeps the whole picture and fills the bars with a blurred, darkened copy of it, so nothing is lost.
func resizeFilter(w, h int, mode string) string {
	if mode == "crop" {
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1", w, h, w, h)
	}
	return fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,boxblur=24:2,eq=brightness=-0.08[bg];"+
		"[0:v]scale=%d:%d:force_original_aspect_ratio=decrease[fg];[bg][fg]overlay=(W-w)/2:(H-h)/2,setsar=1", w, h, w, h, w, h)
}

// resizedName is the copy's file name: promo.mp4 in 9:16 becomes promo-9x16.mp4.
func resizedName(file, aspect string) string {
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	for _, a := range []string{"-9x16", "-16x9", "-1x1", "-4x5"} { // resizing a copy again shouldn't stack the suffixes
		base = strings.TrimSuffix(base, a)
	}
	return base + "-" + strings.ReplaceAll(aspect, ":", "x") + ".mp4"
}

// ResizeVideo makes a copy of a finished video in another frame shape: {file: "out/x.mp4", aspect: "9:16", mode: "fit"|"crop"}.
// It runs in the background like a render and adds the copy to out/, so Publish can send each platform its shape.
func ResizeVideo(r *http.Request) (any, error) {
	b, err := httpapi.Body(r)
	if err != nil {
		return nil, err
	}
	out, err := StartResize(httpapi.PathID(r), store.Str(b["file"]), store.Str(b["aspect"]), store.Str(b["mode"]))
	if err != nil {
		return nil, err
	}
	return map[string]string{"file": out}, nil
}

func StartResize(id int64, file, aspect, mode string) (string, error) {
	size, ok := frameSizes[aspect]
	if !ok {
		return "", httpapi.Bad("pick a shape: 9:16, 16:9, 1:1 or 4:5")
	}
	if mode != "crop" {
		mode = "fit"
	}
	dir := files.ProjectDir(id)
	src := files.Within(dir, file)
	if !strings.HasPrefix(file, "out/") || files.Kind(file) != "video" {
		return "", httpapi.Bad("pick a finished video from out/")
	}
	if _, err := os.Stat(src); err != nil {
		return "", httpapi.Bad("that video isn't there any more")
	}
	if IsRunning(id) {
		return "", httpapi.Bad("something is already running on this project; try again when it's done")
	}
	p, err := store.Row("select status from projects where id=?", id)
	if err != nil {
		return "", err
	}
	out := "out/" + resizedName(file, aspect)
	if out == file {
		return "", httpapi.Bad("that is already the " + aspect + " copy")
	}
	secs := files.MediaSeconds(src)
	args := []string{"-y", "-loglevel", "error", "-progress", "pipe:1", "-nostats", "-i", src}
	if mode == "crop" {
		args = append(args, "-vf", resizeFilter(size[0], size[1], mode), "-map", "0:v:0")
	} else {
		args = append(args, "-filter_complex", resizeFilter(size[0], size[1], mode)+"[v]", "-map", "[v]")
	}
	args = append(args, "-map", "0:a?", "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart", "-f", "mp4", filepath.Join(dir, out)+".part")
	cmd := proc.Command("ffmpeg", args...)
	cmd.Dir = dir
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	var stderr strings.Builder
	cmd.Stderr = &stderr
	label := fmt.Sprintf("%s copy of %s", aspect, filepath.Base(file))
	setStatus(id, "running")
	started := time.Now()
	setRenderProgress(id, &renderProgress{Stage: "Making the " + label, Out: out, Started: started})
	done := make(chan struct{})
	err = Launch(id, cmd, func(err error) {
		pw.Close()
		<-done
		setRenderProgress(id, nil)
		setStatus(id, store.Str(p["status"]))
		if err == nil {
			err = os.Rename(filepath.Join(dir, out)+".part", filepath.Join(dir, out))
		}
		if err != nil {
			os.Remove(filepath.Join(dir, out) + ".part")
			AddEvent(id, "error", map[string]string{"text": "Couldn't make the " + label + ": " + cmp.Or(strings.TrimSpace(textutil.Tail(stderr.String(), 400)), err.Error())})
		} else {
			AddEvent(id, "render", map[string]string{"text": fmt.Sprintf("Made the %s: %s (%s)", label, out, textutil.Clock(time.Since(started).Seconds())), "file": out})
		}
		drainQueue(id)
	})
	if err != nil {
		setStatus(id, store.Str(p["status"]))
		setRenderProgress(id, nil)
		return "", err
	}
	go func() { // ffmpeg's -progress lines: out_time_us=12345678 is how far it has got
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "out_time_us="); ok && secs > 0 {
				if us, err := strconv.ParseFloat(v, 64); err == nil {
					setRenderProgress(id, &renderProgress{Pct: min(99, int(us/1e6/secs*100)), Stage: "Making the " + label, Out: out, Started: started})
				}
			}
		}
		io.Copy(io.Discard, pr)
		close(done)
	}()
	return out, nil
}
