package model

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"codeberg.org/tslocum/gotext"
	. "codeberg.org/tslocum/sriracha/util"
)

type Ban struct {
	ID              int
	IP              string
	Timestamp       int64
	Expire          int64
	Reason          string
	LiftedTimestamp int64
	LiftedReason    string
}

func (b *Ban) Validate() error {
	switch {
	case strings.TrimSpace(b.IP) == "":
		return fmt.Errorf("IP must be set")
	case b.Expire < 0:
		return fmt.Errorf("expiraton must be greater than or equal to zero")
	}
	return nil
}

func (b *Ban) TypeLabel() string {
	if strings.HasPrefix(b.IP, "r ") {
		return fmt.Sprintf("Range %s", strings.ReplaceAll(strings.ReplaceAll(b.IP[2:], `\.`, "."), ".*", "*"))
	}
	return "Address"
}

func (b *Ban) ExpireDate() string {
	if b.Expire == 0 {
		return "Never"
	}
	return time.Unix(b.Expire, 0).Format("2006-01-02 15:04:05 MST")
}

func (b *Ban) AppealID() string {
	if b.IP == "" {
		return ""
	}
	crcHash.Reset()
	crcHash.Write([]byte(fmt.Sprintf("appeal%d", b.Timestamp)))
	crcHash.Write(CRCSalt)
	crcHash.Write([]byte(b.IP))

	crcSum = crcHash.Sum(crcSum[:0])
	base64.RawURLEncoding.Encode(crcBuf, crcSum)
	return string(crcBuf[:5])
}

func (b *Ban) Info() string {
	var info string
	if b.Expire == 0 {
		info += gotext.Get("This ban is permanent.")
	} else {
		info += gotext.Get("This ban will expire at %s.", FormatRawTimestamp(b.Expire))
	}
	if b.Reason != "" {
		info += " " + gotext.Get("Reason: %s", b.Reason)
	}
	return info
}

func (b *Ban) Duration() string {
	if b.LiftedTimestamp == 0 {
		return ""
	}
	return FormatDuration(time.Duration(b.LiftedTimestamp-b.Timestamp) * time.Second)
}
