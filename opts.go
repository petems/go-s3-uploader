package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type options struct {
	BucketName string `json:"bucket_name,omitempty"`
	Source     string `json:"source,omitempty"`
	CacheFile  string `json:"cache_file,omitempty"`
	Region     string `json:"region,omitempty"`
	Profile    string `json:"profile,omitempty"`
	cfgFile    string

	WorkersCount int  `json:"workers_count,omitempty"`
	Encrypt      bool `json:"encrypt,omitempty"`

	dryRun, verbose, quiet,
	doCache, doUpload, saveCfg bool
}

func (o *options) dump(fname string) error {
	f, err := os.Create(fname) // #nosec G304 - file path from user config is expected
	if err != nil {
		return err
	}
	defer func() {
		err2 := f.Close()
		if err == nil {
			err = err2
		} else if err2 != nil {
			err = fmt.Errorf("%w; %w", err, err2)
		}
	}()

	buf, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, "\n"[0])

	_, err = f.Write(buf)

	return err
}

func (o *options) restore(fname string) error {
	f, err := os.Open(fname) // #nosec G304 - file path from user config is expected
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer func() {
		if err2 := f.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()

	tmp := options{}
	dec := json.NewDecoder(f)
	if err = dec.Decode(&tmp); err != nil {
		return err
	}

	o.merge(&tmp)

	return nil
}

func (o *options) merge(other *options) {
	if x := other.WorkersCount; x != 0 {
		o.WorkersCount = x
	}
	if x := other.BucketName; x != "" {
		o.BucketName = x
	}
	if x := other.Source; x != "" {
		o.Source = x
	}
	if x := other.CacheFile; x != "" {
		o.CacheFile = x
	}
	if x := other.Region; x != "" {
		o.Region = x
	}
	if x := other.Profile; x != "" {
		o.Profile = x
	}
	if x := other.Encrypt; x {
		o.Encrypt = x
	}

	// skipping the rest of the fields, they can never come from an unmarshalled file anyway.
}
