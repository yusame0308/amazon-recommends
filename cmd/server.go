package main

import (
	"fmt"
	"net/http"

	"amazon-recommends/internal/repository"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Product struct {
	ProductName string `json:"productName" validate:"required,min=1,max=100"`
	MakerName   string `json:"makerName" validate:"required,min=1,max=50"`
	Price       int    `json:"price" validate:"required,min=1,max=9999999999"`
	Reason      string `json:"reason" validate:"required,min=1,max=100"`
	URL         string `json:"url" validate:"required,url"`
	ASIN        string `json:"asin" validate:"required,len=10,alphanum"`
}

type ProductPatch struct {
	ProductName string `json:"productName" validate:"max=100"`
	MakerName   string `json:"makerName" validate:"max=50"`
	Price       int    `json:"price" validate:"max=9999999999"`
	Reason      string `json:"reason" validate:"max=100"`
	URL         string `json:"url" validate:"omitempty,url"`
}

type CustomValidator struct {
	validator *validator.Validate
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return nil
}

func NewValidator() *CustomValidator {
	return &CustomValidator{
		validator: validator.New(),
	}
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Validator = NewValidator()

	// mysql connection
	dsn := "docker:docker@tcp(127.0.0.1:3306)/amazonRecommends?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err.Error())
	}
	// Migrate the schema
	if err := db.AutoMigrate(&repository.Products{}); err != nil {
		panic(err.Error())
	}

	getData := func(asin string, status bool) (*repository.Products, error) {
		m := new(repository.Products)
		if tx := db.Where("status = ?", status).First(m, "asin = ?", asin); tx.Error != nil {
			return nil, tx.Error
		} else {
			return m, nil
		}
	}

	e.POST("/amazon", func(c echo.Context) error {
		// リクエストを取得
		product := new(Product)
		_ = c.Bind(product)
		// 同じASINが存在しないかチェック
		if m, _ := getData(product.ASIN, true); m != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Error: %s already exists", product.ASIN))
		}
		// バリデーション
		if err := c.Validate(product); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		// Create
		now := time.Now()
		db.Create(&repository.Products{
			ProductName: product.ProductName,
			MakerName:   product.MakerName,
			Price:       product.Price,
			Reason:      product.Reason,
			URL:         product.URL,
			ASIN:        product.ASIN,
			CreatedAt:   now,
			UpdatedAt:   now,
			Status:      true,
		})
		return c.JSON(http.StatusCreated, product)
	})
	e.GET("/amazon/:asin", func(c echo.Context) error {
		// リクエストを取得
		asin := c.Param("asin")
		// データを取得
		m, err := getData(asin, true)
		if err != nil {
			return c.String(http.StatusNotFound, err.Error())
		}

		product := &Product{
			ProductName: m.ProductName,
			MakerName:   m.MakerName,
			Price:       m.Price,
			Reason:      m.Reason,
			URL:         m.URL,
			ASIN:        m.ASIN,
		}
		return c.JSON(http.StatusOK, product)
	})
	e.PUT("/amazon/:asin", func(c echo.Context) error {
		asin := c.Param("asin")
		// データが存在するかチェック
		if _, err := getData(asin, true); err != nil {
			return c.String(http.StatusNotFound, err.Error())
		}
		// リクエストを取得する
		product := new(Product)
		_ = c.Bind(product)
		// バリデーション
		if err := c.Validate(product); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		// Update
		now := time.Now()
		db.Model(repository.Products{}).
			Where("status = ?", true).
			Where("asin = ?", asin).
			Updates(repository.Products{
				ProductName: product.ProductName,
				MakerName:   product.MakerName,
				Price:       product.Price,
				Reason:      product.Reason,
				URL:         product.URL,
				ASIN:        product.ASIN, // IDは変えない方がいいのでは？
				UpdatedAt:   now,
			})
		return c.JSON(http.StatusOK, product)
	})
	e.PATCH("/amazon/:asin", func(c echo.Context) error {
		asin := c.Param("asin")
		// データが存在するかチェック
		if _, err := getData(asin, true); err != nil {
			return c.String(http.StatusNotFound, err.Error())
		}
		// リクエストを取得する
		product := new(ProductPatch)
		_ = c.Bind(product)
		// バリデーション
		if err := c.Validate(product); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		// Update
		now := time.Now()
		db.Model(repository.Products{}).
			Where("status = ?", true).
			Where("asin = ?", asin).
			Updates(repository.Products{
				ProductName: product.ProductName,
				MakerName:   product.MakerName,
				Price:       product.Price,
				Reason:      product.Reason,
				URL:         product.URL,
				UpdatedAt:   now,
			})
		m, _ := getData(asin, true)
		return c.JSON(http.StatusOK, &Product{
			ProductName: m.ProductName,
			MakerName:   m.MakerName,
			Price:       m.Price,
			Reason:      m.Reason,
			URL:         m.URL,
			ASIN:        m.ASIN,
		})
	})
	// 論理削除
	e.PATCH("/amazon/:asin/delete", func(c echo.Context) error {
		// リクエストを取得する
		asin := c.Param("asin")
		// データが存在するかチェック
		if _, err := getData(asin, true); err != nil {
			return c.String(http.StatusNotFound, err.Error())
		}
		// ステータスを無効にする
		db.Model(repository.Products{}).
			Where("asin = ?", asin).
			Update("status", false)
		return c.String(http.StatusNoContent, "")
	})
	// 復元
	e.PATCH("/amazon/:asin/undelete", func(c echo.Context) error {
		// リクエストを取得する
		asin := c.Param("asin")
		// データが存在するかチェック
		m, err := getData(asin, false)
		if err != nil {
			return c.String(http.StatusNotFound, err.Error())
		}
		// ステータスを有効にする
		db.Model(repository.Products{}).Where("asin = ?", asin).Update("status", true)
		return c.JSON(http.StatusOK, &Product{
			ProductName: m.ProductName,
			MakerName:   m.MakerName,
			Price:       m.Price,
			Reason:      m.Reason,
			URL:         m.URL,
			ASIN:        m.ASIN,
		})
	})
	e.Logger.Fatal(e.Start(":1232"))
}
