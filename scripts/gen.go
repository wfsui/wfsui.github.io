package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/mattn/godown"
)

type Option map[string]string

type ArticleSource interface {
	pick(opt Option) []string
}

type Article struct {
	title   string
	content string
}

type InfoQ struct {
}

func (infoq *InfoQ) pick(opt Option) []string {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, _ := http.NewRequest("GET", "https://www.infoq.cn/", nil)
	req.Header.Set("User-Agent", "Golang_Spider_Bot/3.0")
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	return infoq.parse(string(body))
}

func (infoq *InfoQ) parse(html string) []string {
	doc, _ := htmlquery.Parse(strings.NewReader(html))
	list, _ := htmlquery.QueryAll(doc, "//div[contains(@class, 'home-banner-article')]//div[contains(@class, 'article-item')]//a/@href")
	hrefs := []string{}
	for _, n := range list {
		href := htmlquery.SelectAttr(n, "href")
		fmt.Println(href) // output @href value
		hrefs = append(hrefs, href)
	}
	return hrefs
}

type GfwReport struct {
}

func (gfwReport *GfwReport) pick(opt Option) []string {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, _ := http.NewRequest("GET", "https://gfw.report/", nil)
	req.Header.Set("User-Agent", "Golang_Spider_Bot/3.0")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	return gfwReport.parse(string(body))
}

func (gfwReport *GfwReport) parse(html string) []string {
	doc, _ := htmlquery.Parse(strings.NewReader(html))
	list, _ := htmlquery.QueryAll(doc, "//div[contains(@class, 'row')]//li//a/@href")
	hrefs := []string{}
	for _, n := range list {
		href := htmlquery.SelectAttr(n, "href")
		fmt.Println(href) // output @href value
		hrefs = append(hrefs, href[:len(href)-3]+"zh/")
	}
	return hrefs
}

// 1. sources = article links
// 2. request & convert
// 3. pick 10 = headers wordcount images
// 4. add markdown descriptions
func main() {
	sources := []ArticleSource{&InfoQ{}, &GfwReport{}}
	for _, source := range sources {
		links := source.pick(Option{})
		for _, link := range links {
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			req, _ := http.NewRequest("GET", link, nil)
			req.Header.Set("User-Agent", "Golang_Spider_Bot/3.0")
			resp, _ := client.Do(req)
			body, _ := ioutil.ReadAll(resp.Body)
			doc, _ := htmlquery.Parse(strings.NewReader(string(body)))
			tnode := htmlquery.FindOne(doc, "//h1[contains(@class, 'article-title')]")
			title := htmlquery.InnerText(tnode)
			cnode := htmlquery.FindOne(doc, "//div[contains(@class, 'article-preview')]")
			content := htmlquery.OutputHTML(cnode, true)
			f, _ := os.Create(title + ".md")
			w := bufio.NewWriter(f)
			w.WriteString("---")
			w.WriteString("\n")
			w.WriteString("title: \"" + title + "\"")
			w.WriteString("\n")
			now := time.Now()
			w.WriteString("date: " + now.Format("2006-01-02T15:04:05-0700"))
			w.WriteString("\n")
			w.WriteString("---")
			w.WriteString("\n")
			w.WriteString("\n")
			godown.Convert(w, strings.NewReader(content), &godown.Option{TrimSpace: true})
			w.Flush()
		}
	}
}
