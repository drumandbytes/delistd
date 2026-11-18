// Command stylesnap records the computed style of every element on every page
// of a running Delistd UI (desktop light and phone dark), so a CSS or template
// refactor can be compared element by element instead of by eye.
//
//	delistd serve --listen 0.0.0.0 --port 8097 &   # against demo data, never your real ~/.delistd
//	go run ./scripts/stylesnap -base http://localhost:8097 -out before.json
//	... change CSS or templates, rebuild, serve again ...
//	go run ./scripts/stylesnap -base http://localhost:8097 -out after.json
//	go run ./scripts/stylesnap -compare before.json after.json
//
// The comparison lists each changed property with its before and after values
// and how many elements it touches. A box size within 1px is rendering noise.
// Needs Chrome (-chrome <binary>, or one on the PATH); not run in CI.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

const props = `["color","backgroundColor","borderTopColor","borderRightColor","borderBottomColor","borderLeftColor","borderTopWidth","borderRightWidth","borderBottomWidth","borderLeftWidth","borderTopStyle","borderBottomStyle","borderTopLeftRadius","borderTopRightRadius","borderBottomLeftRadius","borderBottomRightRadius","paddingTop","paddingRight","paddingBottom","paddingLeft","marginTop","marginRight","marginBottom","marginLeft","fontSize","fontWeight","fontFamily","fontStyle","lineHeight","letterSpacing","display","flexDirection","flexWrap","flexGrow","flexShrink","alignItems","alignSelf","justifyContent","gap","gridTemplateColumns","position","opacity","boxShadow","textDecorationLine","textTransform","textAlign","overflowX","overflowY","whiteSpace","cursor","zIndex","transitionProperty","animationName"]`

const walk = `(function(props){
  var out=[];
  function path(el){var p=[];while(el&&el.nodeType==1&&el!==document.documentElement){var i=1,s=el;while((s=s.previousElementSibling))i++;p.unshift(el.tagName.toLowerCase()+(el.id?'#'+el.id:'')+':'+i);el=el.parentElement}return p.join('>')}
  document.querySelectorAll('body, body *').forEach(function(el){
    if(['SCRIPT','STYLE','NOSCRIPT','TEMPLATE'].indexOf(el.tagName)>=0)return;
    var cs=getComputedStyle(el),r=el.getBoundingClientRect();
    var v=props.map(function(k){return cs[k]}).join('|')+'|'+Math.round(r.width)+'x'+Math.round(r.height);
    out.push([path(el),(el.getAttribute('class')||''),v]);
  });
  return out;
})(` + props + `)`

type view struct {
	name   string
	w, h   int64
	dark   bool
	mobile bool
}

func main() {
	base := flag.String("base", "http://localhost:8080", "UI address")
	out := flag.String("out", "snap.json", "output file")
	chrome := flag.String("chrome", "", "browser binary")
	compare := flag.Bool("compare", false, "compare two snapshot files given as arguments")
	flag.Parse()
	if *compare {
		if flag.NArg() != 2 {
			fmt.Fprintln(os.Stderr, "usage: stylesnap -compare before.json after.json")
			os.Exit(2)
		}
		if err := compareSnapshots(flag.Arg(0), flag.Arg(1)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Headless, chromedp.DisableGPU)
	if *chrome != "" {
		opts = append(opts, chromedp.ExecPath(*chrome))
	}
	actx, cancelA := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelA()
	ctx, cancel := chromedp.NewContext(actx)
	defer cancel()
	ctx, cancelT := context.WithTimeout(ctx, 10*time.Minute)
	defer cancelT()

	// crawl same-origin GET links two levels deep
	seen := map[string]bool{"/": true}
	queue := []string{"/"}
	for depth := 0; depth < 3 && len(queue) > 0; depth++ {
		var next []string
		for _, p := range queue {
			if err := chromedp.Do(ctx, chromedp.Navigate(*base+p), chromedp.WaitReady("body")); err != nil {
				continue
			}
			hrefs, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](`Array.from(document.querySelectorAll('a[href]')).map(a=>a.getAttribute('href'))`))
			if err != nil {
				continue
			}
			for _, h := range hrefs {
				if !strings.HasPrefix(h, "/") || strings.HasPrefix(h, "//") || strings.Contains(h, "/api/") || strings.Contains(h, "download") ||
					strings.Contains(h, "complete") || strings.Contains(h, "skip") || strings.Contains(h, "/export") || strings.Contains(h, "/static/") {
					continue
				}
				h = strings.SplitN(h, "#", 2)[0]
				if h == "" || seen[h] {
					continue
				}
				seen[h] = true
				next = append(next, h)
			}
		}
		queue = next
	}
	var pages []string
	for p := range seen {
		pages = append(pages, p)
	}
	sort.Strings(pages)
	fmt.Fprintf(os.Stderr, "%d pages\n", len(pages))

	views := []view{{"desktop-light", 1280, 900, false, false}, {"phone-dark", 390, 844, true, true}}
	result := map[string][][3]string{}
	for _, v := range views {
		scheme := map[bool]string{true: "dark", false: "light"}[v.dark]
		if err := chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			if _, err := cdp.Call(ctx, t, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{Width: v.w, Height: v.h, DeviceScaleFactor: 1, Mobile: v.mobile}); err != nil {
				return err
			}
			_, err := cdp.Call(ctx, t, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: scheme}}})
			return err
		})); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		for _, p := range pages {
			theme := `try{localStorage.setItem('delistd-theme','` + scheme + `')}catch(e){}`
			if err := chromedp.Do(ctx, chromedp.Navigate(*base+p), chromedp.WaitReady("body"),
				chromedp.Evaluate[chromedp.Void](theme), chromedp.Navigate(*base+p), chromedp.WaitReady("body"), chromedp.Sleep(700*time.Millisecond)); err != nil {
				fmt.Fprintln(os.Stderr, p, err)
				continue
			}
			rows, err := chromedp.Run(ctx, chromedp.Evaluate[[][3]string](walk))
			if err != nil {
				fmt.Fprintln(os.Stderr, p, err)
				continue
			}
			result[v.name+" "+p] = rows
		}
	}
	b, _ := json.Marshal(result)
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d page views)\n", *out, len(result))
}

var propNames = func() []string {
	var names []string
	if err := json.Unmarshal([]byte(props), &names); err != nil {
		panic(err)
	}
	return append(names, "box")
}()

type change struct{ prop, before, after string }

func compareSnapshots(beforeFile, afterFile string) error {
	load := func(f string) (map[string][][3]string, error) {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var m map[string][][3]string
		return m, json.Unmarshal(b, &m)
	}
	a, err := load(beforeFile)
	if err != nil {
		return err
	}
	b, err := load(afterFile)
	if err != nil {
		return err
	}
	counts := map[change]int{}
	examples := map[change]string{}
	structure := 0
	for view, rowsA := range a {
		byPath := func(rows [][3]string) map[string][3]string {
			m := map[string][3]string{}
			for _, r := range rows {
				m[r[0]] = r
			}
			return m
		}
		ra, rb := byPath(rowsA), byPath(b[view])
		for p, x := range ra {
			y, ok := rb[p]
			if !ok {
				structure++
				continue
			}
			if x[2] == y[2] {
				continue
			}
			va, vb := strings.Split(x[2], "|"), strings.Split(y[2], "|")
			for i := range va {
				if i >= len(vb) || va[i] == vb[i] {
					continue
				}
				if i == len(propNames)-1 && withinOnePixel(va[i], vb[i]) {
					continue
				}
				c := change{propNames[min(i, len(propNames)-1)], clip(va[i]), clip(vb[i])}
				counts[c]++
				if _, ok := examples[c]; !ok {
					examples[c] = view + " " + p + " [" + y[1] + "]"
				}
			}
		}
		for p := range rb {
			if _, ok := ra[p]; !ok {
				structure++
			}
		}
	}
	var list []change
	for c := range counts {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return counts[list[i]] > counts[list[j]] })
	fmt.Printf("%d distinct property changes, %d elements added or removed\n", len(list), structure)
	for _, c := range list {
		fmt.Printf("%5d  %-22s %s -> %s\n       e.g. %s\n", counts[c], c.prop, c.before, c.after, examples[c])
	}
	return nil
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}

func withinOnePixel(a, b string) bool {
	var w1, h1, w2, h2 int
	if _, err := fmt.Sscanf(a, "%dx%d", &w1, &h1); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(b, "%dx%d", &w2, &h2); err != nil {
		return false
	}
	d := func(x, y int) int {
		if x > y {
			return x - y
		}
		return y - x
	}
	return d(w1, w2) <= 1 && d(h1, h2) <= 1
}
