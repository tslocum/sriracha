package model

type Category struct {
	ID          int
	Parent      *Category
	Sort        int
	Name        string
	Description string
	Boards      []*Board    `diff:"-"`
	Categories  []*Category `diff:"-"`
	Overboard   string
}

func (c *Category) HasBoard(id int) bool {
	for _, b := range c.Boards {
		if b.ID == id {
			return true
		}
	}
	return false
}
