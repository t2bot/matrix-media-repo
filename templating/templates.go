package templating

import (
	"fmt"
	"html/template"
	"path"
	"sync"

	"github.com/t2bot/matrix-media-repo/common/config"
)

type templates struct {
	cached map[string]*template.Template
}

var instance *templates
var singletonLock = &sync.Once{}

func getInstance() *templates {
	if instance == nil {
		singletonLock.Do(func() {
			instance = &templates{
				cached: make(map[string]*template.Template),
			}
		})
	}
	return instance
}

func GetTemplate(name string) (*template.Template, error) {
	i := getInstance()
	if v, ok := i.cached[name]; ok {
		return v, nil
	}

	fname := fmt.Sprintf("%s.html", name)
	tmplPath := path.Join(config.Runtime.TemplatesPath, fname)
	t, err := template.New(fname).ParseFiles(tmplPath)
	if err != nil {
		return nil, err
	}

	i.cached[name] = t
	return t, nil
}
