package expert

import (
	"fmt"
	"sort"
)

type Catalog struct {
	all []Expert
	by  map[string]Expert
}

func NewCatalog(experts []Expert) (*Catalog, error) {
	c := &Catalog{by: make(map[string]Expert, len(experts))}
	for _, ex := range experts {
		if err := ex.Validate(); err != nil {
			return nil, err
		}
		if _, exists := c.by[ex.ID]; exists {
			return nil, fmt.Errorf("duplicate expert id %q", ex.ID)
		}
		c.all = append(c.all, ex)
		c.by[ex.ID] = ex
	}
	sortExperts(c.all)
	return c, nil
}

func (c *Catalog) List() []Expert {
	if c == nil {
		return nil
	}
	out := make([]Expert, 0, len(c.all))
	for _, ex := range c.all {
		if ex.Enabled {
			out = append(out, ex)
		}
	}
	return out
}

func (c *Catalog) ListInfo() []Info {
	experts := c.List()
	out := make([]Info, 0, len(experts))
	for _, ex := range experts {
		out = append(out, ex.Info())
	}
	return out
}

func (c *Catalog) All() []Expert {
	if c == nil {
		return nil
	}
	out := make([]Expert, len(c.all))
	copy(out, c.all)
	return out
}

func (c *Catalog) Get(id string) (Expert, bool) {
	if c == nil {
		return Expert{}, false
	}
	ex, ok := c.by[id]
	return ex, ok
}

func (c *Catalog) Resolve(id string) (Expert, error) {
	ex, ok := c.Get(id)
	if !ok {
		return Expert{}, fmt.Errorf("unknown expert %q", id)
	}
	if !ex.Enabled {
		return Expert{}, fmt.Errorf("expert %q is disabled", id)
	}
	return ex, nil
}

func sortExperts(experts []Expert) {
	sort.SliceStable(experts, func(i, j int) bool {
		if experts[i].Category == experts[j].Category {
			return experts[i].ID < experts[j].ID
		}
		return experts[i].Category < experts[j].Category
	})
}
