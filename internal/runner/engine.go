package runner

import (
	"sync"

	"github.com/berkantsoytas/velox/internal/requester"
)

type Config struct {
	URL         string
	Method      string
	Body        []byte
	Headers     map[string]string
	Requests    int
	Concurrency int
}

func Run(cfg Config, req *requester.Requester, resultsChan chan<- requester.Result) {
	jobs := make(chan struct{}, cfg.Requests)

	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()

		for range jobs {
			res := req.Do(cfg.Method, cfg.URL, cfg.Body, cfg.Headers)

			resultsChan <- res
		}
	}

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	for i := 0; i < cfg.Requests; i++ {
		jobs <- struct{}{}
	}

	close(jobs)

	wg.Wait()

	close(resultsChan)
}
