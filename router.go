package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/render"
	"github.com/shxsun/gobuild/models"
)

func InitRouter() {
	var p2id = make(map[string]string)
	var GOROOT = "/Users/skyblue/go"

	m.Get("/", func(r render.Render) {
		r.HTML(200, "index", nil)
	})
	m.Get("/github.com/:account/:proj/:ref/:os/:arch", func(p martini.Params) string {
		var err error
		var id = "uuid" // FIXME
		// create log
		outfd, err := os.OpenFile("log/"+id, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if err != nil {
			lg.Error(err)
		}
		defer outfd.Close()
		// build cmd
		cmd := exec.Command("bin/build", "github.com/"+p["account"]+"/"+p["proj"])
		envs := []string{}
		for k, v := range p {
			envs = append(envs, strings.ToUpper(k)+"="+v)
		}
		url := strings.Join(envs, "/")
		envs = append(envs, "GOROOT="+GOROOT, "BUILD_HOST="+"127.0.0.1:3000", "BUILD_ID="+id)
		cmd.Env = envs
		cmd.Stdout = outfd
		cmd.Stderr = outfd

		p2id[url] = id
		err = cmd.Run()
		return ""
	})

	m.Get("/info/:id/output", func(p martini.Params) string {
		return "unfinished"
	})
	m.Get("/api/:id/finish", func(p martini.Params) string {
		return "unfinished"
	})
	/*m.Get("/github.com/**", func(params martini.Params, r render.Render) {
		r.Redirect("/download/github.com/"+params["_1"], 302)
	})
	*/

	m.Get("/build/**", func(params martini.Params, r render.Render) {
		addr := params["_1"]
		lg.Debug(addr, "END")
		jsDir := strings.Repeat("../", strings.Count(addr, "/")+1)
		r.HTML(200, "build", map[string]string{
			"FullName":       addr,
			"Name":           filepath.Base(addr),
			"DownloadPrefix": options.CDN,
			"Server":         options.Server,
			"WsServer":       options.WsServer + "/websocket",
			"JsDir":          jsDir,
		})
	})
	m.Get("/rebuild/**", func(params martini.Params, r render.Render) {
		addr := params["_1"]
		mu.Lock()
		defer func() {
			mu.Unlock()
			r.Redirect("/build/"+addr, 302) // FIXME: this not good with nginx proxy
		}()
		br := broadcasts[addr]
		lg.Debug("rebuild", addr, "END")
		if br == nil {
			return
		}
		if br.Closed() {
			lg.Debug("rebuild:", addr)
			delete(broadcasts, addr)
		}
		lg.Debug("end rebuild")
	})

	// for autobuild script upload result
	m.Post("/api/update", func(req *http.Request) (int, string) {
		// for secure reason, only accept 127.0.0.1 address
		lg.Warnf("Unexpected request: %s", req.RemoteAddr)
		if !strings.HasPrefix(req.RemoteAddr, "127.0.0.1:") {
			lg.Warnf("Unexpected request: %s", req.RemoteAddr)
			return 200, ""
		}
		project, sha := req.FormValue("p"), req.FormValue("sha")
		lg.Debug(project, sha)

		record := new(models.Project)
		record.Name = project
		record.Project = project // FIXME: delete it
		record.Sha = sha
		err := models.SyncProject(record)
		if err != nil {
			lg.Error(err)
			return 500, err.Error()
		}
		return 200, "OK"
	})

	m.Get("/dl", func(req *http.Request, r render.Render) (code int, body string) {
		os, arch := req.FormValue("os"), req.FormValue("arch") //"windows", "amd64"
		project := req.FormValue("p")                          //"github.com/shxsun/fswatch"
		filename := filepath.Base(project)
		if os == "windows" {
			filename += ".exe"
		}

		// sha should get from db
		//sha := "d1077e2e106489b81c6a404e6951f1fca8967172"
		sha, err := models.GetSha(project)
		if err != nil {
			return 500, err.Error()
		}
		// path like: cdn://project/sha/os_arch/filename
		r.Redirect(options.CDN+"/"+filepath.Join(project, sha, os+"_"+arch, filename), 302)
		return
	})

	m.Get("/dlscript/**", func(params martini.Params) (s string, err error) {
		project := params["_1"]
		t, err := template.ParseFiles("templates/dlscript.sh.tmpl")
		if err != nil {
			lg.Error(err)
			return
		}
		buf := bytes.NewBuffer(nil)
		err = t.Execute(buf, map[string]interface{}{
			"Project": project,
			"Server":  options.Server,
			//"CDN":     options.CDN,
		})
		if err != nil {
			lg.Error(err)
			return
		}
		return string(buf.Bytes()), nil
	})

	m.Get("/download/**", func(params martini.Params, r render.Render) {
		addr := params["_1"]
		basename := filepath.Base(addr)

		files := []string{}
		for _, os := range OS {
			for _, arch := range Arch {
				outfile := fmt.Sprintf("%s/%s/%s_%s_%s", options.CDN, addr, basename, os, arch)
				if os == "windows" {
					outfile += ".exe"
				}
				files = append(files, outfile)
			}
		}
		r.HTML(200, "download", map[string]interface{}{
			"Project": addr,
			"Server":  options.Server,
			"Name":    filepath.Base(addr),
			"CDN":     options.CDN,
			"Files":   files,
		})
	})
}
