package main

import (
	"database/sql"
	"io"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	pusher "github.com/pusher/pusher-http-go"
)

type Photo struct {
	ID  int64  `json:"id"`
	Src string `json:"src"`
}

type PhotoCollection struct {
	Photos []Photo `json:"items"`
}

var client = pusher.Client{
	AppID:   "PUSHER_APP_ID",
	Key:     "PUSHER_APP_KEY",
	Secret:  "PUSHER_APP_SECRET",
	Cluster: "PUSHER_APP_CLUSTER",
	Secure:  true,
}

func initialiseDatabase() *sql.DB {
	dbhost := os.Getenv("DBHOST")
	dbuser := os.Getenv("DBUSER")
	dbpass := os.Getenv("DBPASS")
	dbport := os.Getenv("DBPORT")
	dbname := os.Getenv("DB")
	db, err := sql.Open("mysql", dbuser+":"+dbpass+"@tcp("+dbhost+":"+dbport+")/"+dbname)
	if err != nil {
		log.Println("Connection String failed", err)
	}
	return db
}

func migrateDatabase(db *sql.DB) {
	sql := `
	CREATE TABLE IF NOT EXISTS photos(
			id INTEGER  PRIMARY KEY AUTO_INCREMENT,
			src VARCHAR(255) NOT NULL
	);
`
	_, err := db.Exec(sql)
	if err != nil {
		panic(err)
	}
}

func getPhotos(db *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		rows, err := db.Query("SELECT * FROM photos")
		if err != nil {
			panic(err)
		}
		defer rows.Close()
		result := PhotoCollection{}

		for rows.Next() {
			photo := Photo{}
			err2 := rows.Scan(&photo.ID, &photo.Src)

			if err2 != nil {
				panic(err2)
			}
			result.Photos = append(result.Photos, photo)
		}
		return c.JSON(http.StatusOK, result)
	}
}

func uploadPhoto(db *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		file, err := c.FormFile("file")
		if err != nil {
			return err
		}

		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		filePath := "./public/uploads/" + file.Filename
		fileSrc := "http://127.0.0.1:9000/uploads/" + file.Filename

		dst, err := os.Create(filePath)
		if err != nil {
			panic(err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			panic(err)
		}
		stmt, err := db.Prepare("INSERT INTO photos(src) VALUES(?)")

		if err != nil {
			panic(err)
		}
		defer stmt.Close()
		result, err := stmt.Exec(fileSrc)
		if err != nil {
			panic(err)
		}

		insertedId, err := result.LastInsertId()
		if err != nil {
			panic(err)
		}

		photo := Photo{
			Src: fileSrc,
			ID:  insertedId,
		}
		client.Trigger("photo-stream", "new-photo", photo)
		return c.JSON(http.StatusOK, photo)
	}
}

func main() {
	db := initialiseDatabase()
	migrateDatabase(db)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.File("/", "public/index.html")
	e.GET("/photos", getPhotos(db))
	e.POST("/photos", uploadPhoto(db))
	e.Static("/uploads", "public/uploads")

	e.Logger.Fatal(e.Start(":9000"))
}
