// Copyright 2015 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

// File main.go is intended to be used with go generate.
// It reads the contents of any .lua file ins the scripts
// directory, then it generates a go source file called
// scritps.go which converts the file contents to a string
// and assigns each script to a variable so they can be invoked.

package main

import (
	"bytes"
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/albrow/jobs"
)

var (
	// scriptsPath is the path of the directory which holds lua scripts.
	scriptsPath string
	// destPath is the path to a file where the generated go code will be written.
	destPath string
	// genTmplPath is the path to a .tmpl file which will be used to generate go code.
	genTmplPath string
)

var (
	// scriptContext is a map which is passed in as the context to all lua script templates.
	// It holds the keys for all the different status sets, the names of the sets, and the keys
	// for other constant sets.
	scriptContext = map[string]string{
		"statusSaved":     string(jobs.StatusSaved),
		"statusQueued":    string(jobs.StatusQueued),
		"statusExecuting": string(jobs.StatusExecuting),
		"statusFinished":  string(jobs.StatusFinished),
		"statusFailed":    string(jobs.StatusFailed),
		"statusCancelled": string(jobs.StatusCancelled),
		"statusDestroyed": string(jobs.StatusDestroyed),
		"savedSet":        jobs.StatusSaved.Key(),
		"queuedSet":       jobs.StatusQueued.Key(),
		"executingSet":    jobs.StatusExecuting.Key(),
		"finishedSet":     jobs.StatusFinished.Key(),
		"failedSet":       jobs.StatusFailed.Key(),
		"cancelledSet":    jobs.StatusCancelled.Key(),
		"destroyedSet":    jobs.StatusDestroyed.Key(),
		"timeIndexSet":    jobs.Keys.JobsTimeIndex,
		"jobsTempSet":     jobs.Keys.JobsTemp,
		"activePoolsSet":  jobs.Keys.ActivePools,
	}
)

// script is a representation of a lua script file.
type script struct {
	// VarName is the variable name that the script will be assigned to in the generated go code.
	VarName string
	// RawSrc is the contents of the original .lua file, which is a template.
	RawSrc string
	// Src is the the result of executing RawSrc as a template.
	Src string
}

func init() {
	// Use build to find the directory where this file lives. This always works as
	// long as you have go installed, even if you have multiple GOPATHs or are using
	// dependency management tools.
	pkg, err := build.Import("github.com/albrow/jobs", "", build.FindOnly)
	if err != nil {
		panic(err)
	}
	// Configure the required paths
	scriptsPath = filepath.Join(pkg.Dir, "scripts")
	destPath = filepath.Clean(filepath.Join(scriptsPath, "..", "scripts.go"))
	genTmplPath = filepath.Join(scriptsPath, "scripts.go.tmpl")
}

func main() {
	scripts, err := findScripts(scriptsPath)
	if err != nil {
		panic(err)
	}
	if err := generateFile(scripts, genTmplPath, destPath); err != nil {
		panic(err)
	}
}

// findScripts finds all the .lua script files in the given path
// and creates a script object for each one. It returns a slice of
// scripts or an error if there was a problem reading any of the files.
func findScripts(path string) ([]*script, error) {
	filenames, err := filepath.Glob(filepath.Join(path, "*.lua"))
	if err != nil {
		return nil, err
	}
	scripts := []*script{}
	for _, filename := range filenames {
		script := script{
			VarName: convertUnderscoresToCamelCase(strings.TrimSuffix(filepath.Base(filename), ".lua")) + "Script",
		}
		src, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		script.RawSrc = string(src)
		scripts = append(scripts, &script)
	}
	return scripts, nil
}

// convertUnderscoresToCamelCase converts a string of the form
// foo_bar_baz to fooBarBaz.
func convertUnderscoresToCamelCase(s string) string {
	if len(s) == 0 {
		return ""
	}
	result := ""
	shouldUpper := false
	for _, char := range s {
		if char == '_' {
			shouldUpper = true
			continue
		}
		if shouldUpper {
			result += strings.ToUpper(string(char))
		} else {
			result += string(char)
		}
		shouldUpper = false
	}
	return result
}

// generateFile generates go source code and writes to
// the source file located at dest (creating it if needed).
// It executes the template located at tmplFile with scripts
// as the context.
func generateFile(scripts []*script, tmplFile string, dest string) error {
	// Treat the contents of the script file as a template and execute
	// it with scriptsContext (a map of constant keys) as the data.
	buf := bytes.NewBuffer([]byte{})
	for _, script := range scripts {
		scriptTmpl, err := template.New("script").Parse(script.RawSrc)
		if err != nil {
			return err
		}
		buf.Reset()
		if err := scriptTmpl.Execute(buf, scriptContext); err != nil {
			return err
		}
		script.Src = buf.String()
	}
	// Now generate the go source code by executing genTmpl. The generated
	// code uses the Src property of the scripts as the argument to redis.NewScript.
	genTmpl, err := template.ParseFiles(tmplFile)
	if err != nil {
		return err
	}
	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	return genTmpl.Execute(destFile, scripts)
}
