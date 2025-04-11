package sriracha

type BoardType int

const (
	TypeImageboard BoardType = 0
	TypeForum      BoardType = 1
)

type Board struct {
	ID          int
	Dir         string
	Name        string
	Description string
	Type        BoardType
}
