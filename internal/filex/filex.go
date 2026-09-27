// Package filex provides shared file and stream utilities for size-limited reads
// and DoS protection limits across CONGA packages.
package filex

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrFileSizeLimitExceeded indicates a file or stream exceeded the maximum allowed size limit.
var ErrFileSizeLimitExceeded = errors.New("file size limit exceeded")

// ReadLimited reads the file at path up to maxBytes, returning ErrFileSizeLimitExceeded if exceeded.
func ReadLimited(path string, maxBytes int64) ([]byte, error) {
	//nolint:gosec // File path validated by caller.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = file.Close() //nolint:errcheck // Best-effort file close on return.
	}()

	if st, err := file.Stat(); err == nil && st.Mode().IsRegular() && st.Size() > maxBytes {
		return nil, fmt.Errorf("%w: file size %d bytes exceeds maximum limit of %d bytes", ErrFileSizeLimitExceeded, st.Size(), maxBytes)
	}

	data, err := ReadLimitedReader(file, maxBytes)
	if err != nil {
		if errors.Is(err, ErrFileSizeLimitExceeded) {
			return nil, fmt.Errorf("%w: file size exceeds maximum limit of %d bytes", ErrFileSizeLimitExceeded, maxBytes)
		}

		return nil, err
	}

	return data, nil
}

// ReadLimitedReader reads from r up to maxBytes, returning ErrFileSizeLimitExceeded if exceeded.
func ReadLimitedReader(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: exceeds maximum limit of %d bytes", ErrFileSizeLimitExceeded, maxBytes)
	}

	return data, nil
}
