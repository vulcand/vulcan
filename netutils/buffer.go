package netutils

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// TODO: make configurable
const MEMORY_BUFFER_LIMIT = 1024 * 1024 * 1024

// Constraints:
//  - Implements io.Reader
//  - Implements Seek(0, 0)
//	- Designed for Write once, Read many times.
type multiReaderSeek struct {
	length  int64
	readers []io.ReadSeeker
	mr      io.Reader
	cleanup CleanupFunc
}

type CleanupFunc func() error

func MultiReaderSeeker(cleanup CleanupFunc, readers ...io.ReadSeeker) *multiReaderSeek {
	ior := make([]io.Reader, len(readers))
	for i, arg := range readers {
		ior[i] = arg.(io.Reader)
	}

	return &multiReaderSeek{length: -1,
		readers: readers,
		mr:      io.MultiReader(ior...),
		cleanup: cleanup}
}

func (mr *multiReaderSeek) Close() (err error) {
	if mr.cleanup != nil {
		return mr.cleanup()
	}
	return nil
}

func (mr *multiReaderSeek) Read(p []byte) (n int, err error) {
	return mr.mr.Read(p)
}

func (mr *multiReaderSeek) Len() (int64, error) {
	if mr.length >= 0 {
		return mr.length, nil
	}

	var totalLen int64
	for _, reader := range mr.readers {
		switch reader.(type) {
		case *bytes.Reader:
			b := reader.(*bytes.Reader)
			totalLen += int64(b.Len())
		case *os.File:
			// STAT
			f := reader.(*os.File)
			st, err := f.Stat()
			if err != nil {
				return 0, err
			}
			totalLen += st.Size()
		default:
			return 0, fmt.Errorf("multiReaderSeek: type for Len: %s", reader)
		}
	}

	mr.length = totalLen

	return mr.length, nil
}

func (mr *multiReaderSeek) Seek(offset int64, whence int) (int64, error) {
	// TODO: implement other whence
	// TODO: implement real offsets

	if whence != 0 {
		return 0, fmt.Errorf("multiReaderSeek: unsupported whence")
	}

	if offset != 0 {
		return 0, fmt.Errorf("multiReaderSeek: unsupported offset")
	}

	for _, seeker := range mr.readers {
		seeker.Seek(0, 0)
	}

	ior := make([]io.Reader, len(mr.readers))
	for i, arg := range mr.readers {
		ior[i] = arg.(io.Reader)
	}
	mr.mr = io.MultiReader(ior...)

	return 0, nil
}

func NewBodyBuffer(input io.Reader) (*multiReaderSeek, error) {
	var f *os.File
	ior := make([]io.ReadSeeker, 0, 2)
	lr := &io.LimitedReader{input, MEMORY_BUFFER_LIMIT}
	buffer, err := ioutil.ReadAll(lr)

	if err != nil {
		return nil, err
	}

	ior = append(ior, bytes.NewReader(buffer))
	if lr.N <= 0 {
		f, err := ioutil.TempFile("", "vulcan-bodies-")
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(f, input)
		if err != nil {
			return nil, err
		}
		f.Seek(0, 0)
		ior = append(ior, f)
	}

	mrs := MultiReaderSeeker(func() error {
		if f != nil {
			os.Remove(f.Name())
			f.Close()
		}
		return nil
	}, ior...)

	return mrs, nil
}
