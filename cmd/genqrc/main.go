// XXX: The documentation is duplicated here and in the the doc variable
// below. Update both at the same time.

// Command genqrc packs resource files into the Go binary.
//
// Usage: genqrc [options] <subdir1> [<subdir2> ...]
//
// The genqrc tool packs all resource files under the provided subdirectories into
// a single qrc.go file that may be built into the generated binary. Bundled files
// may then be loaded by Go or QML code under the URL "qrc:///some/path", where
// "some/path" matches the original path for the resource file locally.
//
// For example, the following will load a .qml file from the resource pack, and
// that file may in turn reference other content (code, images, etc) in the pack:
//
//     component, err := engine.LoadFile("qrc://path/to/file.qml")
//
// Starting with Go 1.4, this tool may be conveniently run by the "go generate"
// subcommand by adding a line similar to the following one to any existent .go
// file in the project (assuming the subdirectories ./code/ and ./images/ exist):
//
//     //go:generate genqrc code images
//
// Then, just run "go generate" to update the qrc.go file.
//
// During development, the generated qrc.go can repack the filesystem content at
// runtime to avoid the process of regenerating the qrc.go file and rebuilding the
// application to test every minor change made. Runtime repacking is enabled by
// setting the QRC_REPACK environment variable to 1:
//
//     export QRC_REPACK=1
//
// This does not update the static content in the qrc.go file, though, so after
// the changes are performed, genqrc must be run again to update the content that
// will ship with built binaries.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/i-home/qml"
)

const doc = `
Usage: genqrc [options] <subdir1> [<subdir2> ...]

The genqrc tool packs all resource files under the provided subdirectories into
a single qrc.go file that may be built into the generated binary. Bundled files
may then be loaded by Go or QML code under the URL "qrc:///some/path", where
"some/path" matches the original path for the resource file locally.

For example, the following will load a .qml file from the resource pack, and
that file may in turn reference other content (code, images, etc) in the pack:

    component, err := engine.LoadFile("qrc://path/to/file.qml")

Starting with Go 1.4, this tool may be conveniently run by the "go generate"
subcommand by adding a line similar to the following one to any existent .go
file in the project (assuming the subdirectories ./code/ and ./images/ exist):

    //go:generate genqrc code images

Then, just run "go generate" to update the qrc.go file.

During development, the generated qrc.go can repack the filesystem content at
runtime to avoid the process of regenerating the qrc.go file and rebuilding the
application to test every minor change made. Runtime repacking is enabled by
setting the QRC_REPACK environment variable to 1:

    export QRC_REPACK=1

This does not update the static content in the qrc.go file, though, so after
the changes are performed, genqrc must be run again to update the content that
will ship with built binaries.
`

// XXX: The documentation is duplicated here and in the the package comment
// above. Update both at the same time.

var packageName = flag.String("package", "main", "package name that qrc.go will be under (not needed for go generate)")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", doc)
		flag.PrintDefaults()
	}
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	subdirs := flag.Args()
	if len(subdirs) == 0 {
		return fmt.Errorf("must provide at least one subdirectory path")
	}

	var rp qml.ResourcesPacker

	for _, subdir := range flag.Args() {
		err := filepath.Walk(subdir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			rp.Add(filepath.ToSlash(path), data)
			return nil
		})
		if err != nil {
			return err
		}
	}

	resdata := rp.Pack().Bytes()

	f, err := os.Create("qrc.go")
	if err != nil {
		return err
	}
	defer f.Close()

	data := templateData{
		PackageName:   *packageName,
		SubDirs:       subdirs,
		ResourcesData: resdata,
	}

	// $GOPACKAGE is set automatically by go generate.
	if pkgname := os.Getenv("GOPACKAGE"); pkgname != "" {
		data.PackageName = pkgname
	}

	return tmpl.Execute(f, data)
}

type templateData struct {
	PackageName   string
	SubDirs       []string
	ResourcesData []byte
}

func buildTemplate(name, content string) *template.Template {
	return template.Must(template.New(name).Parse(content))
}

var tmpl = buildTemplate("qrc.go", `package {{.PackageName}}

// This file is automatically generated by github.com/i-home/qml/cmd/genqrc

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/i-home/qml"
)

func init() {
	var r *qml.Resources
	var err error
	if os.Getenv("QRC_REPACK") == "1" {
		err = qrcRepackResources()
		if err != nil {
			panic("cannot repack qrc resources: " + err.Error())
		}
		r, err = qml.ParseResources(qrcResourcesRepacked)
	} else {
		r, err = qml.ParseResourcesString(qrcResourcesData)
	}
	if err != nil {
		panic("cannot parse bundled resources data: " + err.Error())
	}
	qml.LoadResources(r)
}

func qrcRepackResources() error {
	subdirs := {{printf "%#v" .SubDirs}}
	var rp qml.ResourcesPacker
	for _, subdir := range subdirs {
		err := filepath.Walk(subdir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			rp.Add(filepath.ToSlash(path), data)
			return nil
		})
		if err != nil {
			return err
		}
	}
	qrcResourcesRepacked = rp.Pack().Bytes()
	return nil
}

var qrcResourcesRepacked []byte
var qrcResourcesData = {{printf "%q" .ResourcesData}}
`)
