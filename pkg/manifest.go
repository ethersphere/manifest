// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manifest

type Entry struct {
	// temp type
	reference string // swarm reference to the file
	// temp type
	fileInfo     string  // file or dir info akin to one given on file system
	headers      Headers // HTTP response headers, e.g. content type
	accessParams AccessControlParams
	crsParams    CRSparams // info needed for erasure coding
}

// BoS says this should be:
// define type headers
//     `content.type  [segment size]byte`
// how can this be reconciled?
type Headers map[string]string

// unkown for now
type AccessControlParams struct {
}

// unknown for now
type CRSparams struct {
}
