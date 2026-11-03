// Package proc starts a command together with everything it goes on to spawn, and ends the whole lot at once, so
// Stop really stops a run: ffmpeg, Chrome under HyperFrames, the real binary behind a Node wrapper.
//
// Unix does this with a process group (Setpgid, then a signal to -pid). Windows has no such thing, so Start puts the
// command in a job object: whatever it starts joins the job, and the job ends them all together. The job also ends
// its processes if 37Studio itself dies, which a process group does not.
//
// Use Start for anything the app can stop, and StartDetached for the few runs meant to outlive the command that
// began them (transcription keeps going after the agent's turn ends). A detached run is reached by pid later, with
// Alive and TerminatePID, since nothing of it is held in memory.
package proc
