package server

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/bufbuild/protovalidate-go"
	"github.com/hashicorp/go-hclog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"grpc/pkg/product_v1"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ProductFromCSV struct {
	Name  string
	Price float64
}
type Product struct {
	Name         string                 `bson:"name"`
	Price        float64                `bson:"price"`
	PriceChanges int64                  `bson:"price_changes"`
	LastUpdate   *timestamppb.Timestamp `bson:"last_update"`
}
type ProductServer struct {
	product_v1.UnimplementedProductV1Server
	log         hclog.Logger
	mongoClient *mongo.Client
	validator   *protovalidate.Validator
}

func NewProductServer(log hclog.Logger, mongoURI string) (*ProductServer, error) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}
	v, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create protovalidate: %v", err)
	}
	return &ProductServer{
		log:         log,
		mongoClient: client,
		validator:   v,
	}, nil
}

func (s *ProductServer) Fetch(ctx context.Context, req *product_v1.URL) (*emptypb.Empty, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.validator.Validate(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	productsFromCSV, _ := fetchProductsFromCSV(req.Value)
	collection := s.mongoClient.Database("your_db_name").Collection("products")
	for _, product := range productsFromCSV {
		filter := bson.M{"name": product.Name}
		update := bson.M{"$set": bson.M{"price": product.Price, "last_update": timestamppb.Now()}}
		update["$inc"] = bson.M{"price_changes": 1}
		_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update product in MongoDB: %v", err)
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *ProductServer) List(ctx context.Context, req *product_v1.ListRequest) (*product_v1.ListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	sortField := req.SortBy.GetValue()
	pageNumber := req.PageNumber
	pageSize := req.PageSize

	opts := options.Find()
	opts.SetSkip((pageNumber - 1) * pageSize)
	opts.SetLimit(pageSize)
	sortOrder := 1
	if !req.SortAscending.GetValue() {
		sortOrder = -1
	}

	collation := options.Collation{
		Locale:   "en",
		Strength: 2,
	}

	opts.SetSort(bson.D{{Key: sortField, Value: sortOrder}}).SetCollation(&collation)

	collection := s.mongoClient.Database("your_db_name").Collection("products")
	cursor, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list products from MongoDB: %v", err)
	}
	defer cursor.Close(ctx)

	var products []*product_v1.Product
	for cursor.Next(ctx) {
		var product Product
		if err := cursor.Decode(&product); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to decode product: %v", err)
		}

		products = append(products, &product_v1.Product{
			Name:         product.Name,
			Price:        product.Price,
			PriceChanges: product.PriceChanges,
			LastUpdate:   product.LastUpdate,
		})
	}

	resp := &product_v1.ListResponse{
		Products: products,
	}
	return resp, nil
}
func fetchProductsFromCSV(url string) ([]*Product, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	reader := csv.NewReader(response.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var nameIndex, priceIndex int
	header := records[0]
	for i, column := range header {
		if strings.EqualFold(column, "product_name") {
			nameIndex = i
		} else if strings.EqualFold(column, "price") {
			priceIndex = i
		}
	}

	var products []*Product
	for _, record := range records[1:] {
		price, err := strconv.ParseFloat(record[priceIndex], 64)
		if err != nil {
			continue
		}
		if price != 0 {
			product := &Product{
				Name:  record[nameIndex],
				Price: price,
			}
			products = append(products, product)
		}
	}
	return products, nil
}
