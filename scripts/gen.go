package main

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/mattn/godown"
	"gopkg.in/yaml.v2"
)

type Conf struct {
	Subscribe map[string][]struct {
		Link  string `yaml:"link"`
		Hrefs string `yaml:"hrefs"`
	} `yaml:"subscribe"`
	Source map[string]struct {
		Title   string `yaml:"title"`
		Content string `yaml:"content"`
		Tags    string `yaml:"tags"`
	} `yaml:"source"`
}

var client = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func main() {
	yamlFile, err := ioutil.ReadFile("tasks.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	var conf Conf
	err = yaml.Unmarshal(yamlFile, &conf)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}
	jsonConf, err := json.Marshal(&conf)
	if err != nil {
		log.Fatalf("Marshal: %v", err)
	}
	log.Printf("Loaded Config: %s", string(jsonConf))
	for tag, subscribe := range conf.Subscribe {
		hrefs := []string{}
		for _, link := range subscribe {
			if link.Hrefs == "" {
				hrefs = append(hrefs, link.Link)
			} else {
				req, _ := http.NewRequest("GET", link.Link, nil)
				req.Header.Set("User-Agent", "Golang_Spider_Bot/3.0")
				resp, _ := client.Do(req)
				doc, _ := htmlquery.Parse(resp.Body)
				list, _ := htmlquery.QueryAll(doc, link.Hrefs)
				for _, n := range list {
					href := htmlquery.SelectAttr(n, "href")
					hrefs = append(hrefs, href)
				}
			}
		}
		for _, href := range hrefs {
			req, _ := http.NewRequest("GET", href, nil)
			req.Header.Set("User-Agent", "Golang_Spider_Bot/3.0")
			resp, _ := client.Do(req)
			doc, _ := htmlquery.Parse(resp.Body)

			url, err := url.Parse(href)
			if err != nil {
				log.Fatalf("Invalid Url: %s Error: %v", href, err)
			}
			source, ok := conf.Source[url.Host]
			if !ok {
				log.Fatalf("No source find for %s", url.Host)
			}
			tnode := htmlquery.FindOne(doc, source.Title)
			title := htmlquery.InnerText(tnode)
			cnode := htmlquery.FindOne(doc, source.Content)
			content := htmlquery.OutputHTML(cnode, true)
			tnodes, _ := htmlquery.QueryAll(doc, source.Tags)
			tags := []string{tag}
			for _, tnode := range tnodes {
				t := htmlquery.InnerText(tnode)
				tags = append(tags, t)
			}
			f, _ := os.Create(title + ".md")
			w := bufio.NewWriter(f)
			w.WriteString("---")
			w.WriteString("\n")
			w.WriteString("title: \"" + title + "\"")
			w.WriteString("\n")
			now := time.Now()
			w.WriteString("date: " + now.Format("2006-01-02T15:04:05-0700"))
			w.WriteString("\n")
			w.WriteString("tags: [" + strings.Join(tags, ", ") + "]")
			w.WriteString("\n")
			w.WriteString("---")
			w.WriteString("\n")
			w.WriteString("\n")
			godown.Convert(w, strings.NewReader(content), &godown.Option{TrimSpace: true})
			w.Flush()
		}
	}
}
