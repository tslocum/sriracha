// Package util provides constants, variables and functions related to Sriracha.
package util

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/mail"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/constraints"
)

const NewDirPermission = 0755
const NewFilePermission = 0644

var (
	AlphaNumericAndSymbols = regexp.MustCompile(`^[0-9A-Za-z_\-]+$`)
	FileNamePattern        = regexp.MustCompile(`^[0-9A-Za-z_\-.]+$`)
	FilePathPattern        = regexp.MustCompile(`^[0-9A-Za-z_\-/.]+$`)

	QuotePattern = regexp.MustCompile(`^&gt;(.*)$`)

	RefLinkPattern   = regexp.MustCompile(`&gt;&gt;([0-9]+)`)
	BoardLinkPattern = regexp.MustCompile(`&gt;&gt;&gt;\/([0-9A-Za-z_-]+)?\/?`)

	URLPattern     = regexp.MustCompile(`(?i)(((f|ht)tp(s)?:\/\/)[-a-zA-Zа-яА-Я()0-9@%\!_+.,~#?&;:|\'\/=]+)`)
	FixURLPattern1 = regexp.MustCompile(`(?i)\(\<a href\=\"(.*)\)"\ target\=\"\_blank\">(.*)\)\<\/a>`)
	FixURLPattern2 = regexp.MustCompile(`(?i)\<a href\=\"(.*)\."\ target\=\"\_blank\">(.*)\.\<\/a>`)
	FixURLPattern3 = regexp.MustCompile(`(?i)\<a href\=\"(.*)\,"\ target\=\"\_blank\">(.*)\,\<\/a>`)
)

func ParseInt(v string) int {
	i, err := strconv.Atoi(v)
	if err == nil && i > 0 {
		return i
	}
	return 0
}

func ParseInt64(v string) int64 {
	i, err := strconv.ParseInt(v, 10, 64)
	if err == nil && i > 0 {
		return i
	}
	return 0
}

func ParseFloat(v string) float64 {
	i, err := strconv.ParseFloat(v, 64)
	if err == nil && i > 0 {
		return i
	}
	return 0
}

func FormString(r *http.Request, key string) string {
	return strings.TrimSpace(r.FormValue(key))
}

func FormMultiString(r *http.Request, key string) []string {
	formKeys := make([]string, len(r.Form))
	var i int
	for key := range r.Form {
		formKeys[i] = key
		i++
	}
	sort.Slice(formKeys, func(i, j int) bool {
		return formKeys[i] < formKeys[j]
	})
	var values []string
	for _, formKey := range formKeys {
		formValues := r.Form[formKey]
		if strings.HasPrefix(formKey, key+"_") {
			for _, v := range formValues {
				if strings.TrimSpace(v) == "" {
					continue
				}
				values = append(values, v)
			}
		}
	}
	return values
}

func FormInt(r *http.Request, key string) int {
	v, err := strconv.Atoi(FormString(r, key))
	if err == nil && v >= 0 {
		return v
	}
	return 0
}

func FormInt64(r *http.Request, key string) int64 {
	v, err := strconv.ParseInt(FormString(r, key), 10, 64)
	if err == nil && v >= 0 {
		return v
	}
	return 0
}

func FormNegInt(r *http.Request, key string) int {
	v, err := strconv.Atoi(FormString(r, key))
	if err == nil {
		return v
	}
	return 0
}

func FormBool(r *http.Request, key string) bool {
	return FormInt(r, key) == 1
}

func FormRange[T constraints.Integer](r *http.Request, key string, min T, max T) T {
	v := FormNegInt(r, key)
	if v >= int(min) && v <= int(max) {
		return T(v)
	}
	return min
}

func PathInt(r *http.Request, prefix string) int {
	pathValue := PathString(r, prefix)
	if pathValue != "" {
		v, err := strconv.Atoi(pathValue)
		if err == nil && v > 0 {
			return v
		}
	}
	return 0
}

func PathString(r *http.Request, prefix string) string {
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return ""
	}
	return strings.TrimPrefix(r.URL.Path, prefix)
}

func ParseEmail(address string) string {
	a, err := mail.ParseAddress(address)
	if err != nil {
		return ""
	}
	return a.Address
}

func MIMEToExt(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/pjpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/png":
		return "png"
	default:
		return ""
	}
}

func FormatTimestamp(timestamp int64) template.HTML {
	utcDate := template.HTML(time.Unix(timestamp, 0).In(time.UTC).Format("2006-01-02T15:04:05Z"))
	return `<time datetime="` + utcDate + `" title="` + utcDate + `">` + template.HTML(time.Unix(timestamp, 0).Format("2006/01/02<wbr>(Mon)<wbr>15:04:05")) + `</time>`
}

func FormatRawTimestamp(timestamp int64) string {
	return time.Unix(timestamp, 0).Format("2006/01/02(Mon)15:04:05")
}

func FormatYYYYMMDD(timestamp int64) string {
	return time.Unix(timestamp, 0).Format("2006/01/02")
}

func FormatDateInput(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Format("2006/01/02 15:04")
}

func FormatFileSize(size int64) string {
	v := float64(size)
	for _, unit := range []string{"", "K", "M", "G", "T", "P", "E", "Z"} {
		if math.Abs(v) < 1024.0 {
			return fmt.Sprintf("%.0f%sB", v, unit)
		}
		v /= 1024.0
	}
	return fmt.Sprintf("%.0fYB", v)
}

func FormatDuration(d time.Duration) string {
	var out string
	hours := int(d.Hours())
	years := hours / (24 * 365)
	if years > 0 {
		out += fmt.Sprintf("%dy ", years)
		hours %= 24 * 365
	}
	days := hours / 24
	if days > 0 {
		out += fmt.Sprintf("%dd ", days)
	}
	d %= 24 * time.Hour
	switch {
	case d >= time.Hour:
		return out + fmt.Sprintf("%.0fh", d.Hours())
	case d >= time.Minute:
		return out + fmt.Sprintf("%.0fm", d.Minutes())
	default:
		seconds := int(d.Seconds())
		if seconds == 0 {
			return out
		}
		return out + fmt.Sprintf("%ds", seconds)
	}
}
