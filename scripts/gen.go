package main

type Option map[string]string

type ArticleSource interface {
	pick(opt Option) []string
}

type InfoQ struct {
}

func (infoq *InfoQ) pick(opt Option) []string {

}

// 1. sources = article links
// 2. request & convert
// 3. pick 10 = headers wordcount images
// 4. add markdown descriptions
func main() {

}
