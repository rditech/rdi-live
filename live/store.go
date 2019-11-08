// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package live

import (
	"encoding/gob"
	"os"

	"github.com/gorilla/sessions"
)

var Store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

func init() {
	gob.Register(map[string]interface{}{})
}
