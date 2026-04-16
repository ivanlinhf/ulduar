package presentationdialect

const (
	VersionV1      = "v1"
	SlideSize16By9 = "16:9"
)

type SlideLayout string

const (
	LayoutTitle        SlideLayout = "title"
	LayoutSection      SlideLayout = "section"
	LayoutTitleBullets SlideLayout = "title_bullets"
	LayoutTwoColumn    SlideLayout = "two_column"
	LayoutTable        SlideLayout = "table"
	LayoutClosing      SlideLayout = "closing"
)

type BlockType string

const (
	BlockTypeParagraph    BlockType = "paragraph"
	BlockTypeBulletList   BlockType = "bullet_list"
	BlockTypeNumberedList BlockType = "numbered_list"
	BlockTypeTable        BlockType = "table"
	BlockTypeQuote        BlockType = "quote"
)

type Document struct {
	Version   string  `json:"version"`
	SlideSize string  `json:"slideSize,omitempty"`
	Slides    []Slide `json:"slides"`
}

type Slide struct {
	Layout   SlideLayout `json:"layout"`
	Title    string      `json:"title"`
	Subtitle string      `json:"subtitle,omitempty"`
	Blocks   []Block     `json:"blocks,omitempty"`
	Columns  []Column    `json:"columns,omitempty"`
}

type Column struct {
	Heading string  `json:"heading,omitempty"`
	Blocks  []Block `json:"blocks"`
}

type Block struct {
	Type        BlockType  `json:"type"`
	Text        string     `json:"text,omitempty"`
	Items       []string   `json:"items,omitempty"`
	Header      []string   `json:"header,omitempty"`
	Rows        [][]string `json:"rows,omitempty"`
	Attribution string     `json:"attribution,omitempty"`
}
