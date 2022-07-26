package gee

import (
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"
)

type HandlerFunc func(c *Context)

type Engine struct {
	*RouterGroup
	router        *router
	groups        []*RouterGroup
	htmlTemplates *template.Template // for html render
	funcMap       template.FuncMap   // for html render
}

type RouterGroup struct {
	prefix string

	parent *RouterGroup

	middlewares []HandlerFunc // 添加在这个路由分组上的中间件

	engine *Engine // 所有的路由分组都共享一个 engine 实例
}

func (group *RouterGroup) Use(middlewares ...HandlerFunc) {
	group.middlewares = append(group.middlewares, middlewares...)
}

// 在 group 下面加一个 newGroup, 增量前缀是 prefix, 返回创建的新 group
func (group *RouterGroup) Group(prefix string) *RouterGroup {
	engine := group.engine

	newGroup := &RouterGroup{
		engine: engine,
		prefix: group.prefix + prefix,
		parent: group,
	}

	engine.groups = append(engine.groups, newGroup)

	return newGroup
}

func New() *Engine {
	engine := &Engine{router: newRouter()}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	return engine
}

func (group *RouterGroup) addRoute(method string, compact string, handler HandlerFunc) {
	pattern := group.prefix + compact
	log.Printf("Route %4s - %s", method, pattern)
	group.engine.router.addRoute(method, pattern, handler)
}

func (group *RouterGroup) GET(pattern string, handler HandlerFunc) {
	group.addRoute("GET", pattern, handler)
}

func (group *RouterGroup) POST(pattern string, handler HandlerFunc) {
	group.addRoute("POST", pattern, handler)
}

func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var middlewares []HandlerFunc

	for _, group := range engine.groups {
		if strings.HasPrefix(req.URL.Path, group.prefix) {
			middlewares = append(middlewares, group.middlewares...)
		}
	}

	c := newContext(w, req)
	c.handlers = middlewares
	c.engine = engine
	engine.router.handle(c)
}

func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, engine)
}

func (group *RouterGroup) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc {
	absolutePath := path.Join(group.prefix, relativePath)

	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))

	return func(c *Context) {
		file := c.Param("filepath")

		//check if the target file exists
		if _, err := fs.Open(file); err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		fileServer.ServeHTTP(c.Writer, c.Req)
	}
}

func (group *RouterGroup) Static(relativePath string, root string) {
	handler := group.createStaticHandler(relativePath, http.Dir(root))
	urlPattern := path.Join(relativePath, "/*filepath")
	//register GET handler
	group.GET(urlPattern, handler)
}

func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
	engine.htmlTemplates = template.Must(template.New("").Funcs(engine.funcMap).ParseGlob(pattern))
}
