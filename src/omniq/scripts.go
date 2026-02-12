package omniq

import (
	"embed"
	"fmt"
	"os"
)

//go:embed core/scripts/*.lua
var embeddedScripts embed.FS

type ScriptDef struct {
	SHA string
	Src string
}

type OmniqScripts struct {
	Enqueue        				ScriptDef
	Reserve        				ScriptDef
	AckSuccess     				ScriptDef
	AckFail        				ScriptDef
	PromoteDelayed 				ScriptDef
	ReapExpired    				ScriptDef
	Heartbeat      				ScriptDef
	Pause          				ScriptDef
	Resume         				ScriptDef
	RetryFailed   	 			ScriptDef
	RetryFailedBatch   	 		ScriptDef
	RemoveJob 					ScriptDef
	RemoveJobsBatch				ScriptDef
	CheckCompletionInit			ScriptDef
	CheckCompletionDecrement	ScriptDef
}

func DefaultScriptsDir() string {
	return ""
}

func LoadScripts(r RedisLike, scriptsDir string) (OmniqScripts, error) {
	loadOne := func(name string) (ScriptDef, error) {
		var src []byte
		var err error

		if scriptsDir == "" {
			src, err = embeddedScripts.ReadFile("core/scripts/" + name)
		} else {
			src, err = os.ReadFile(scriptsDir + "/" + name)
		}
		if err != nil {
			return ScriptDef{}, fmt.Errorf("read script %s: %w", name, err)
		}

		sha, err := r.ScriptLoad(string(src))
		if err != nil {
			return ScriptDef{}, err
		}

		return ScriptDef{
			SHA: sha,
			Src: string(src),
		}, nil
	}

	var err error
	s := OmniqScripts{}

	if s.Enqueue, err = loadOne("enqueue.lua"); err != nil { return s, err }
	if s.Reserve, err = loadOne("reserve.lua"); err != nil { return s, err }
	if s.AckSuccess, err = loadOne("ack_success.lua"); err != nil { return s, err }
	if s.AckFail, err = loadOne("ack_fail.lua"); err != nil { return s, err }
	if s.PromoteDelayed, err = loadOne("promote_delayed.lua"); err != nil { return s, err }
	if s.ReapExpired, err = loadOne("reap_expired.lua"); err != nil { return s, err }
	if s.Heartbeat, err = loadOne("heartbeat.lua"); err != nil { return s, err }
	if s.Pause, err = loadOne("pause.lua"); err != nil { return s, err }
	if s.Resume, err = loadOne("resume.lua"); err != nil { return s, err }
	if s.RetryFailed, err = loadOne("retry_failed.lua"); err != nil { return s, err }
	if s.RetryFailedBatch, err = loadOne("retry_failed_batch.lua"); err != nil { return s, err }
	if s.RemoveJob, err = loadOne("remove_job.lua"); err != nil { return s, err }
	if s.RemoveJobsBatch, err = loadOne("remove_jobs_batch.lua"); err != nil { return s, err }
	if s.CheckCompletionInit, err = loadOne("check_completion_init.lua"); err != nil { return s, err }
	if s.CheckCompletionDecrement, err = loadOne("check_completion_decrement.lua"); err != nil { return s, err }

	return s, nil
}
