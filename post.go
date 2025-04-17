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

func (p *Post) setFileAndThumb(fileExt string) {
	postUploadFileLock.Lock()
	defer postUploadFileLock.Unlock()

	fileID := time.Now().UnixNano()
	fileIDString := fmt.Sprintf("%d", fileID)

	p.File = fileIDString + "." + fileExt
	p.Thumb = fileIDString + "s." + fileExt
}

func (p *Post) setFileAttributes(buf []byte, name string) error {
	checksum := sha512.Sum384(buf)
	p.FileHash = base64.StdEncoding.EncodeToString(checksum[:])

	p.FileOriginal = name

	p.FileSize = int64(len(buf))

	imgConfig, _, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("unsupported filetype")
	}
	p.FileWidth, p.FileHeight = imgConfig.Width, imgConfig.Height
	return nil
}

func (p *Post) createThumbnail(b *Board, buf []byte, mimeType string, thumbPath string) error {
	thumbImg, err := resizeImage(b, bytes.NewReader(buf), mimeType)
	if err != nil {
		return err
	}

	bounds := thumbImg.Bounds()
	p.ThumbWidth, p.ThumbHeight = bounds.Dx(), bounds.Dy()

	err = writeImage(thumbImg, mimeType, thumbPath)
	if err != nil {
		return fmt.Errorf("unsupported filetype")
	}
	return nil
}

func (p *Post) loadForm(r *http.Request, b *Board, rootDir string) error {
	p.Parent = formInt(r, "parent")

	p.Name = formString(r, "name")
	p.Email = formString(r, "email")
	p.Subject = formString(r, "subject")
	p.Message = html.EscapeString(formString(r, "message"))

	formFile, formFileHeader, err := r.FormFile("file")
	if err != nil || formFileHeader == nil {
		return nil
	}

	buf, err := io.ReadAll(formFile)
	if err != nil {
		log.Fatal(err)
	}

	mimeType := http.DetectContentType(buf)

	fileExt := mimeToExt(mimeType)
	if fileExt == "" {
		return fmt.Errorf("unsupported filetype")
	}

	p.setFileAndThumb(fileExt)

	err = p.setFileAttributes(buf, formFileHeader.Filename)
	if err != nil {
		return err
	}

	srcPath := filepath.Join(rootDir, b.Dir, "src", p.File)
	thumbPath := filepath.Join(rootDir, b.Dir, "thumb", p.Thumb)

	err = p.createThumbnail(b, buf, mimeType, thumbPath)
	if err != nil {
		return err
	}

	err = os.WriteFile(srcPath, buf, 0600)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (p *Post) ThreadID() int {
	if p.Parent == 0 {
		return p.ID
	}
	return p.Parent
}

func (p *Post) FileSizeLabel() string {
	return formatFileSize(p.FileSize)
}

func (p *Post) TimestampLabel() string {
	return time.Unix(p.Timestamp, 0).Format("2006/01/02(Mon)15:04:05")
}

func mimeToExt(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/png":
		return "png"
	default:
		return ""
	}
}

func resizeImage(b *Board, r io.Reader, mimeType string) (image.Image, error) {
	var img image.Image
	var err error
	switch mimeType {
	case "image/jpeg":
		img, err = jpeg.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("unsupported filetype")
		}
	case "image/gif":
		img, err = gif.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("unsupported filetype")
		}
	case "image/png":
		img, err = png.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("unsupported filetype")
		}
	}
	return resize.Thumbnail(uint(b.ThumbWidth), uint(b.ThumbHeight), img, resize.Lanczos3), nil
}

func writeImage(img image.Image, mimeType string, filePath string) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	switch mimeType {
	case "image/jpeg":
		err = jpeg.Encode(file, img, nil)
		if err != nil {
			return fmt.Errorf("unsupported filetype")
		}
	case "image/gif":
		err = gif.Encode(file, img, nil)
		if err != nil {
			return fmt.Errorf("unsupported filetype")
		}
	case "image/png":
		err = png.Encode(file, img)
		if err != nil {
			return fmt.Errorf("unsupported filetype")
		}
	}
	return nil
}
