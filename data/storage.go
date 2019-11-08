// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/proio-org/go-proio"
)

type RunObject struct {
	Name string
}

func ListResourceRuns(ctx context.Context, urlString, credentials string) (runs []*RunObject, err error) {
	var thisUrl *url.URL
	thisUrl, err = url.Parse(urlString)
	if err != nil {
		return
	}

	switch thisUrl.Scheme {
	case "gs":
		runs, err = ListGcsRuns(
			ctx,
			thisUrl.Host,
			strings.TrimLeft(thisUrl.Path, "/"),
			[]byte(credentials),
		)
	case "file":
		var files []string
		files, err = filepath.Glob(fmt.Sprintf("%v/%v/*.proio", thisUrl.Host, strings.TrimLeft(thisUrl.Path, "/")))
		for _, file := range files {
			runs = append(runs, &RunObject{Name: path.Base(file)})
		}
	default:
		err = errors.New("bad url scheme")
	}
	return
}

func GetReader(ctx context.Context, urlString, credentials string) (reader *proio.Reader, err error) {
	var thisUrl *url.URL
	thisUrl, err = url.Parse(urlString)
	if err != nil {
		return
	}

	switch thisUrl.Scheme {
	case "gs":
		reader, err = CreateGcsReader(
			ctx,
			thisUrl.Host,
			strings.TrimLeft(thisUrl.Path, "/"),
			[]byte(credentials),
		)
	case "file":
		reader, err = proio.Open(filepath.Clean(fmt.Sprintf("%v/%v", thisUrl.Host, strings.TrimLeft(thisUrl.Path, "/"))))
	default:
		err = errors.New("bad url scheme")
	}
	return
}

func GetWriter(ctx context.Context, urlString, credentials string) (writer *proio.Writer, err error) {
	var thisUrl *url.URL
	thisUrl, err = url.Parse(urlString)
	if err != nil {
		return
	}

	switch thisUrl.Scheme {
	case "gs":
		writer, err = CreateGcsWriter(
			ctx,
			thisUrl.Host,
			strings.TrimLeft(thisUrl.Path, "/"),
			[]byte(credentials),
		)
	case "file":
		writer, err = proio.Create(filepath.Clean(fmt.Sprintf("%v/%v", thisUrl.Host, thisUrl.Path)))
	default:
		err = errors.New("bad url scheme")
	}

	return
}
