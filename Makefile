run:
	go mod tidy && go mod download && go run ./cmd/app
compose-up:
	docker-compose up -d mongodb prometheus grafana

grpc-load-test:
	ghz \
		--proto api/product_v1/product.proto \
		--call product_v1.ProductV1.List \
		--data '{"page_number": "1", "page_size": "100", "sort_ascending": true, "sort_by": "name"}' \
		--rps 1000 \
		--total 30000 \
		--insecure \
		localhost:50051

grpc-error-load-test:
	ghz \
		--proto api/product_v1/product.proto \
		--call product_v1.ProductV1.List \
		--data '{"page_number": "0", "page_size": "100", "sort_ascending": true, "sort_by": "name"}' \
        --rps 100 \
		--total 300 \
		--insecure \
		localhost:50051