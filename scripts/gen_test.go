package main

type Option map[string]string

type ArticleSource interface {
	pick(opt Option) []string
}

type InfoQ struct {
}

func (infoq *InfoQ) pick(opt Option) []string {
	resp, err := http.Get("https://www.infoq.cn/")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

}

// 1. sources = article links
// 2. request & convert
// 3. pick 10 = headers wordcount images
// 4. add markdown descriptions
func main() {

}
