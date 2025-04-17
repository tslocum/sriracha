package sriracha

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nfnt/resize"
)

type Post struct {
	ID           int
	Parent       int
	Timestamp    int64
	Bumped       int64
	IP           string
	Name         string
	Tripcode     string
	Email        string
	NameBlock    string
	Subject      string
	Message      string
	Password     string
	File         string
	FileHash     string
	FileOriginal string
	FileSize     int64
	FileWidth    int
	FileHeight   int
	Thumb        string
	ThumbWidth   int
	ThumbHeight  int
	Moderated    int
	Stickied     int
	Locked       int
}

var postUploadFileLock = &sync.Mutex{}

func (p *Post) loadForm(r *http.Request, b *Board, rootDir string) error {
	p.Parent = formInt(r, "parent")

	p.Name = formString(r, "name")
	p.Email = formString(r, "email")
	p.Subject = formString(r, "subject")
	p.Message = html.EscapeString(formString(r, "message"))

	formFile, formFileHeader, err := r.FormFile("file")
	if err == nil && formFileHeader != nil {
		buf, err := io.ReadAll(formFile)
		if err != nil {
			log.Fatal(err)
		}

		var fileExt string
		mimeType := http.DetectContentType(buf)
		switch mimeType {
		case "image/jpeg":
			fileExt = "jpg"
		case "image/gif":
			fileExt = "gif"
		case "image/png":
			fileExt = "png"
		default:
			return fmt.Errorf("unsupported filetype")
		}

		checksum := sha512.Sum384(buf)
		p.FileHash = base64.StdEncoding.EncodeToString(checksum[:])

		p.FileOriginal = formFileHeader.Filename

		p.FileSize = int64(len(buf))

		imgConfig, _, err := image.DecodeConfig(bytes.NewReader(buf))
		if err != nil {
			return fmt.Errorf("unsupported filetype")
		}
		p.FileWidth, p.FileHeight = imgConfig.Width, imgConfig.Height

		postUploadFileLock.Lock()

		fileID := time.Now().UnixNano()
		fileIDString := fmt.Sprintf("%d", fileID)

		p.File = fileIDString + "." + fileExt
		p.Thumb = fileIDString + "s." + fileExt

		postUploadFileLock.Unlock()

		var img image.Image
		switch mimeType {
		case "image/jpeg":
			img, err = jpeg.Decode(bytes.NewReader(buf))
			if err != nil {
				return fmt.Errorf("unsupported filetype")
			}
		case "image/gif":
			img, err = gif.Decode(bytes.NewReader(buf))
			if err != nil {
				return fmt.Errorf("unsupported filetype")
			}
		case "image/png":
			img, err = png.Decode(bytes.NewReader(buf))
			if err != nil {
				return fmt.Errorf("unsupported filetype")
			}
		}
		thumbImg := resize.Thumbnail(uint(b.ThumbWidth), uint(b.ThumbHeight), img, resize.Lanczos3)

		bounds := thumbImg.Bounds()
		p.ThumbWidth, p.ThumbHeight = bounds.Dx(), bounds.Dy()

		srcPath := filepath.Join(rootDir, b.Dir, "src", p.File)
		thumbPath := filepath.Join(rootDir, b.Dir, "thumb", p.Thumb)

		err = os.WriteFile(srcPath, buf, 0600)
		if err != nil {
			log.Fatal(err)
		}

		thumb, err := os.OpenFile(thumbPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatal(err)
		}
		defer thumb.Close()

		switch mimeType {
		case "image/jpeg":
			err = jpeg.Encode(thumb, thumbImg, nil)
			if err != nil {
				return fmt.Errorf("unsupported filetype")
			}
		case "image/gif":
			err = gif.Encode(thumb, thumbImg, nil)
			if err != nil {
				return fmt.Errorf("unsupported filetype")
			}
		case "image/png":
			err = png.Encode(thumb, thumbImg)
			if err != nil {
				return fmt.Errorf("unsupported filetype")
			}
		}
	}
	return nil
}

func (p *Post) ThreadID() int {
	if p.Parent != 0 {
		return p.Parent
	}
	return p.ID
}

func (p *Post) FileSizeLabel() string {
	return formatFileSize(p.FileSize)
}

func (p *Post) TimestampLabel() string {
	return time.Unix(p.Timestamp, 0).Format("2006/01/02(Mon)15:04:05")
}
