package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"syscall"
)

type connectionConfig struct {
	Version   int    `json:"version"`
	Identity  string `json:"identity"`
	Target    string `json:"target"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Socket    string `json:"socket"`
	TokenFile string `json:"tokenFile"`
	State     string `json:"state"`
}

func privateRead(path string, v any) error {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return errors.New("configuration must be a private regular file")
	}
	d := json.NewDecoder(io.LimitReader(f, 64<<10))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("one configuration object required")
	}
	return nil
}

// Never overwrite existing credential/config files, including on repeated enroll.
func createPrivate(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return err
}
