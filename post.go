package sriracha

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
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

		srcPath := filepath.Join(rootDir, b.Dir, "src", p.File)
		thumbPath := filepath.Join(rootDir, b.Dir, "thumb", p.Thumb)

		err = os.WriteFile(srcPath, buf, 0600)
		if err != nil {
			log.Fatal(err)
		}

		// TODO thumb
		err = os.WriteFile(thumbPath, buf, 0600)
		if err != nil {
			log.Fatal(err)
		}

		postUploadFileLock.Unlock()
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
