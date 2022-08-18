package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v9"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	userpb "github.com/vbua/userManagement/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log"
	"net"
	"os"
	"time"
)

type userServiceServer struct {
	conn *sql.DB
	userpb.UnimplementedUserServiceServer
}

func (u *userServiceServer) DeleteUser(ctx context.Context, req *userpb.DeleteUserRequest) (*userpb.DeleteUserResponse, error) {
	db := u.conn
	stmt, err := db.Prepare("DELETE FROM users WHERE id=$1")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	_, err = stmt.Exec(req.Id)
	if err != nil {
		return nil, err
	}
	return &userpb.DeleteUserResponse{Success: true}, nil
}

func logf(msg string, a ...interface{}) {
	fmt.Printf(msg, a...)
	fmt.Println()
}

func (u *userServiceServer) CreateUser(ctx context.Context, req *userpb.CreateUserRequest) (*userpb.CreateUserResponse, error) {
	w := &kafka.Writer{
		Addr:        kafka.TCP("kafka:9092"),
		Topic:       "user_adds",
		Balancer:    &kafka.LeastBytes{},
		Logger:      kafka.LoggerFunc(logf),
		ErrorLogger: kafka.LoggerFunc(logf),
	}
	defer w.Close()
	db := u.conn
	var id int
	err := db.QueryRow("INSERT INTO users (name, created_at) VALUES ($1, $2) RETURNING id",
		req.User.Name, req.User.CreatedAt.AsTime()).Scan(&id)
	if err != nil {
		res, err := json.Marshal(struct {
			Timestamp uint64 `json:"timestamp"`
			UserId    uint64 `json:"user_id"`
			Success   bool   `json:"success"`
		}{uint64(time.Now().UTC().Unix()), 0, false})
		if err != nil {
			return nil, err
		}
		err = w.WriteMessages(context.Background(),
			kafka.Message{
				Value: res,
			},
		)
		if err != nil {
			return nil, err
		}

		return nil, err
	}

	res, err := json.Marshal(struct {
		Timestamp uint64 `json:"timestamp"`
		UserId    uint64 `json:"user_id"`
		Success   bool   `json:"success"`
	}{uint64(time.Now().UTC().Unix()), uint64(id), true})
	if err != nil {
		return nil, err
	}
	fmt.Println(string(res))
	err = w.WriteMessages(context.Background(),
		kafka.Message{
			Value: res,
		},
	)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return &userpb.CreateUserResponse{Id: uint64(id)}, nil
}

func (u *userServiceServer) ListUsers(ctx context.Context, req *userpb.ListUsersRequest) (*userpb.ListUsersResponse, error) {
	ctxB := context.Background()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})

	cachedUsers, err := redisClient.Get(ctxB, "users").Bytes()
	if err != nil {
		fmt.Println("Couldn't get users from redis", err)
		db := u.conn
		rows, err := db.Query("SELECT * FROM users")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var createdAt time.Time
		var users []*userpb.User
		for rows.Next() {
			var user userpb.User
			err = rows.Scan(&user.Id, &user.Name, &createdAt)
			user.CreatedAt = timestamppb.New(createdAt)
			users = append(users, &user)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		cachedUsers, err = json.Marshal(users)
		if err != nil {
			return nil, err
		}
		err = redisClient.Set(ctxB, "users", cachedUsers, 1*time.Minute).Err()
		if err != nil {
			return nil, err
		}
		fmt.Println("Saving users to redis")
		return &userpb.ListUsersResponse{Users: users}, nil
	}
	var users []*userpb.User
	err = json.Unmarshal(cachedUsers, &users)
	if err != nil {
		return nil, err
	}
	fmt.Println("cached Users", users)
	return &userpb.ListUsersResponse{Users: users}, nil
}

func main() {
	err := godotenv.Load(".env")
	dbinfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"), os.Getenv("DB_NAME"))
	db, err := sql.Open("postgres", dbinfo)
	if err != nil {
		log.Fatalf("Connection couldn't be opened: %v", err)
	}
	defer db.Close()
	err = db.Ping()
	if err != nil {
		log.Fatalf("Connection hasn't been established, ping didn't work: %v", err)
	}
	lis, err := net.Listen("tcp", ":5000")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	userpb.RegisterUserServiceServer(grpcServer, &userServiceServer{conn: db})
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve the listener: %v", err)
	}
}
