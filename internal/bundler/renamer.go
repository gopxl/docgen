package bundler

// Renamer rewrites the filepath.
type Renamer interface {
	Rename(p string) string
}

type nullRenamer struct {
}

func (r *nullRenamer) Rename(p string) string {
	return p
}

type CompositeRenamer struct {
	rs []Renamer
}

func NewCompositeRewriter(rs ...Renamer) *CompositeRenamer {
	return &CompositeRenamer{
		rs: rs,
	}
}

func (c *CompositeRenamer) Rename(p string) string {
	for _, r := range c.rs {
		p = r.Rename(p)
	}
	return p
}
