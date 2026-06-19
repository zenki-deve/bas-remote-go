package basremote

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gofrs/flock"
)

const endpoint = "https://bablosoft.com"

type engineService struct {
	scriptName string
	scriptDir  string
	engineDir  string
	zipDir     string
	exeDir     string
	process    *exec.Cmd
	lock       *flock.Flock
}

func newEngineService(opts *Options) *engineService {
	return &engineService{
		scriptName: opts.ScriptName,
		scriptDir:  filepath.Join(opts.WorkingDir, "run", opts.ScriptName),
		engineDir:  filepath.Join(opts.WorkingDir, "engine"),
	}
}

// initialize fetches script metadata and resolves engine/zip directories.
func (e *engineService) initialize() error {
	url := fmt.Sprintf("%s/scripts/%s/properties", endpoint, e.scriptName)
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Quick check for success field before full Script parse.
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}

	script, err := NewScript(body)
	if err != nil {
		return err
	}
	if !script.IsExist() {
		return ErrScriptNotExist
	}
	if !script.IsSupported() {
		return ErrScriptNotSupported
	}

	e.zipDir = filepath.Join(e.engineDir, script.EngineVersion())
	e.exeDir = filepath.Join(e.scriptDir, script.Hash()[:5])
	return nil
}

// start downloads (if needed), extracts (if needed) and launches the engine.
func (e *engineService) start(port int) error {
	bits := 64
	if runtime.GOARCH == "386" {
		bits = 32
	}
	zipName := fmt.Sprintf("FastExecuteScriptProtected.x%d", bits)
	urlName := fmt.Sprintf("FastExecuteScriptProtected%d", bits)
	zipPath := filepath.Join(e.zipDir, zipName+".zip")

	if _, err := os.Stat(e.zipDir); os.IsNotExist(err) {
		if err := os.MkdirAll(e.zipDir, 0o755); err != nil {
			return err
		}
		if err := e.downloadExecutable(zipPath, zipName, urlName); err != nil {
			return err
		}
	}

	if _, err := os.Stat(e.exeDir); os.IsNotExist(err) {
		if err := os.MkdirAll(e.exeDir, 0o755); err != nil {
			return err
		}
		if err := e.extractExecutable(zipPath); err != nil {
			return err
		}
	}

	if err := e.startProcess(port); err != nil {
		return err
	}
	e.clearRunDirectory()
	return nil
}

func (e *engineService) downloadExecutable(zipPath, zipName, urlName string) error {
	engineVersion := filepath.Base(e.zipDir)
	url := fmt.Sprintf("http://downloads.bablosoft.com/distr/%s/%s/%s.zip",
		urlName, engineVersion, zipName)

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 16*1024)
	_, err = io.CopyBuffer(f, resp.Body, buf)
	return err
}

func (e *engineService) extractExecutable(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		destPath := filepath.Join(e.exeDir, filepath.Clean(f.Name))
		// Prevent zip-slip.
		if !strings.HasPrefix(destPath, filepath.Clean(e.exeDir)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		if err := extractFile(f, destPath); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func (e *engineService) startProcess(port int) error {
	exePath := filepath.Join(e.exeDir, "FastExecuteScript.exe")
	cmd := exec.Command(exePath,
		fmt.Sprintf("--remote-control-port=%d", port),
		"--remote-control",
	)
	cmd.Dir = e.exeDir
	if err := cmd.Start(); err != nil {
		return err
	}
	e.process = cmd

	lockPath := e.lockPath(e.exeDir)
	fl := flock.New(lockPath)
	if _, err := fl.TryLock(); err != nil {
		return err
	}
	e.lock = fl
	return nil
}

func (e *engineService) clearRunDirectory() {
	entries, err := os.ReadDir(e.scriptDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(e.scriptDir, entry.Name())
		lockPath := e.lockPath(dirPath)
		if !isLocked(lockPath) {
			_ = os.RemoveAll(dirPath)
		}
	}
}

func (e *engineService) lockPath(dir string) string {
	return filepath.Join(dir, ".lock")
}

// close kills the engine process and releases the file lock.
func (e *engineService) close() error {
	if e.process != nil && e.process.Process != nil {
		_ = e.process.Process.Kill()
	}
	if e.lock != nil {
		_ = e.lock.Unlock()
	}
	return nil
}

// isLocked returns true if the given lock file is held by another process.
func isLocked(lockPath string) bool {
	fl := flock.New(lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return true
	}
	if locked {
		_ = fl.Unlock()
		return false
	}
	return true
}
