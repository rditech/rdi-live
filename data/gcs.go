// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package data

import (
	"context"

	"cloud.google.com/go/storage"
	"github.com/proio-org/go-proio"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

func ListGcsRuns(ctx context.Context, bucket, prefix string, credentials []byte) ([]*RunObject, error) {
	client, err := storage.NewClient(
		ctx,
		option.WithCredentialsJSON(credentials),
	)
	if err != nil {
		return nil, err
	}

	var runList []*RunObject

	bucketHandle := client.Bucket(bucket)
	it := bucketHandle.Objects(ctx, &storage.Query{Prefix: prefix})
	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		runList = append(runList, &RunObject{Name: objAttrs.Name})
	}

	return runList, nil
}

func CreateGcsReader(ctx context.Context, bucket, name string, credentials []byte) (*proio.Reader, error) {
	client, err := storage.NewClient(
		ctx,
		option.WithCredentialsJSON(credentials),
	)
	if err != nil {
		return nil, err
	}

	objectReader, err := client.Bucket(bucket).Object(name).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	proioReader := proio.NewReader(objectReader)
	proioReader.DeferUntilClose(func() { objectReader.Close() })
	proioReader.DeferUntilClose(func() { client.Close() })
	return proioReader, nil
}

func CreateGcsWriter(ctx context.Context, bucket, name string, credentials []byte) (*proio.Writer, error) {
	client, err := storage.NewClient(
		ctx,
		option.WithCredentialsJSON(credentials),
	)
	if err != nil {
		return nil, err
	}

	objectWriter := client.Bucket(bucket).Object(name).NewWriter(ctx)
	if err != nil {
		return nil, err
	}
	proioWriter := proio.NewWriter(objectWriter)
	proioWriter.DeferUntilClose(objectWriter.Close)
	proioWriter.DeferUntilClose(client.Close)
	return proioWriter, nil
}
