// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package live

type SetStringer interface {
	SetString(key, value string)
}

type Status struct {
	Keys       []string
	StringData map[string]string
}

func (s *Status) SetString(key, value string) {
	if s.StringData == nil {
		s.StringData = make(map[string]string)
	}
	if _, ok := s.StringData[key]; !ok {
		s.Keys = append(s.Keys, key)
	}
	s.StringData[key] = value
}
