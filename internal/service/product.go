package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	pb "gin-kratos-gorm/api/product/v1"
	"gin-kratos-gorm/internal/biz"
	"gin-kratos-gorm/internal/biz/model"
	"gin-kratos-gorm/internal/conf"
	"gin-kratos-gorm/internal/data/persistence"
	"gin-kratos-gorm/internal/pkg/apperr"

	"github.com/dtm-labs/client/dtmcli"
	"github.com/dtm-labs/client/dtmgrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProductService struct {
	pb.UnimplementedProductServer
	uc   *biz.ProductUsecase
	repo *persistence.ProductRepo
	cfg  *conf.Bootstrap
}

func NewProductService(uc *biz.ProductUsecase, repo *persistence.ProductRepo, cfg *conf.Bootstrap) *ProductService {
	return &ProductService{uc: uc, repo: repo, cfg: cfg}
}

func (s *ProductService) GetProductDetail(ctx context.Context, req *pb.GetProductDetailReq) (*pb.GetProductDetailReply, error) {
	spu, err := s.uc.Get(ctx, int64(req.GetId()))
	if err != nil {
		return nil, err
	}
	return toProductDetail(spu), nil
}

func (s *ProductService) ListProducts(ctx context.Context, req *pb.ListProductsReq) (*pb.ListProductsReply, error) {
	spus, total, err := s.uc.List(ctx, biz.ProductFilter{
		Page:       int(req.GetPage()),
		PageSize:   int(req.GetPageSize()),
		Keyword:    req.GetKeyword(),
		CategoryID: req.GetCategoryId(),
		OnSale:     req.OnSale,
	})
	if err != nil {
		return nil, err
	}
	reply := &pb.ListProductsReply{
		Total:    total,
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	}
	for _, spu := range spus {
		reply.Products = append(reply.Products, toProductSummary(spu))
	}
	return reply, nil
}

func (s *ProductService) CreateProduct(ctx context.Context, req *pb.CreateProductReq) (*pb.GetProductDetailReply, error) {
	spu := &model.ProductSpu{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		CategoryID:  int64(req.GetCategoryId()),
		BrandID:     int64(req.GetBrandId()),
		MainImage:   req.GetMainImage(),
		IsOnSale:    true,
	}
	for _, sku := range req.GetSkus() {
		spu.Skus = append(spu.Skus, model.ProductSku{
			SkuCode:   sku.GetSkuCode(),
			AttrValue: sku.GetAttrValue(),
			Price:     sku.GetPrice(),
			Stock:     sku.GetStock(),
		})
	}
	created, err := s.uc.Create(ctx, spu)
	if err != nil {
		return nil, err
	}
	return toProductDetail(created), nil
}

func (s *ProductService) UpdateProduct(ctx context.Context, req *pb.UpdateProductReq) (*pb.GetProductDetailReply, error) {
	spu, err := s.uc.Update(ctx, &model.ProductSpu{
		ID:          int64(req.GetId()),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		CategoryID:  int64(req.GetCategoryId()),
		BrandID:     int64(req.GetBrandId()),
		MainImage:   req.GetMainImage(),
	})
	if err != nil {
		return nil, err
	}
	return toProductDetail(spu), nil
}

func (s *ProductService) SetOnSale(ctx context.Context, req *pb.SetOnSaleReq) (*pb.SimpleResponse, error) {
	if err := s.uc.SetOnSale(ctx, int64(req.GetId()), req.GetOnSale()); err != nil {
		return nil, err
	}
	return &pb.SimpleResponse{Msg: "ok"}, nil
}

func (s *ProductService) AdjustStock(ctx context.Context, req *pb.AdjustStockReq) (*pb.AdjustStockReply, error) {
	stock, err := s.uc.AdjustStock(ctx, int64(req.GetSkuId()), req.GetDelta())
	if err != nil {
		return nil, err
	}
	return &pb.AdjustStockReply{SkuId: int64(req.GetSkuId()), Stock: stock}, nil
}

func (s *ProductService) GetSku(ctx context.Context, req *pb.GetSkuReq) (*pb.SkuDetail, error) {
	sku, err := s.uc.GetSku(ctx, req.GetSkuId())
	if err != nil {
		return nil, err
	}
	return &pb.SkuDetail{
		Id:      sku.SkuID,
		SpuId:   sku.SpuID,
		SkuName: sku.SkuName,
		SkuPic:  sku.SkuPic,
		Price:   sku.Price,
		Stock:   sku.Stock,
		OnSale:  sku.OnSale,
	}, nil
}

// ReduceStock supports both deployments:
//   - local mode: a normal repository transaction;
//   - DTM mode: a branch barrier so the idempotency row, stock update and DTM
//     barrier record are committed atomically. The barrier path uses raw SQL
//     because DTM hands us a *sql.Tx, not a *gorm.DB.
func (s *ProductService) ReduceStock(ctx context.Context, req *pb.ReduceStockReq) (*pb.SimpleResponse, error) {
	if req.GetOrderSn() == "" || req.GetSkuId() <= 0 || req.GetNum() <= 0 {
		return nil, apperr.Invalid("order_sn, sku_id and num are required")
	}
	if barrier, err := dtmgrpc.BarrierFromGrpc(ctx); err == nil && barrier != nil {
		if err := s.dtmBarrierDeduct(ctx, barrier, req); err != nil {
			return nil, err
		}
		return &pb.SimpleResponse{Msg: "success"}, nil
	}
	if err := s.repo.DeductStock(ctx, req.GetOrderSn(), req.GetSkuId(), req.GetNum()); err != nil {
		return nil, err
	}
	return &pb.SimpleResponse{Msg: "success"}, nil
}

func (s *ProductService) CompensateStock(ctx context.Context, req *pb.ReduceStockReq) (*pb.SimpleResponse, error) {
	if req.GetOrderSn() == "" || req.GetSkuId() <= 0 || req.GetNum() <= 0 {
		return nil, apperr.Invalid("order_sn, sku_id and num are required")
	}
	if barrier, err := dtmgrpc.BarrierFromGrpc(ctx); err == nil && barrier != nil {
		if err := s.dtmBarrierRestore(ctx, barrier, req); err != nil {
			return nil, err
		}
		return &pb.SimpleResponse{Msg: "done"}, nil
	}
	if err := s.repo.RestoreStock(ctx, req.GetOrderSn(), req.GetSkuId(), req.GetNum()); err != nil {
		return nil, err
	}
	return &pb.SimpleResponse{Msg: "done"}, nil
}

func (s *ProductService) dtmBarrierDeduct(ctx context.Context, barrier *dtmcli.BranchBarrier, req *pb.ReduceStockReq) error {
	sqlDB, err := s.repo.DB(ctx).DB()
	if err != nil {
		return apperr.Internal("get sql db", err)
	}
	logID := fmt.Sprintf("deduct:%s:%d", req.GetOrderSn(), req.GetSkuId())
	return barrier.CallWithDB(sqlDB, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			"INSERT IGNORE INTO stock_logs (id, order_sn, sku_id, num, action, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			logID, req.GetOrderSn(), req.GetSkuId(), req.GetNum(), "deduct", time.Now(),
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil // idempotent duplicate
		}
		res, err = tx.Exec(
			"UPDATE product_skus SET stock = stock - ? WHERE id = ? AND stock >= ? AND deleted_at IS NULL",
			req.GetNum(), req.GetSkuId(), req.GetNum(),
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return status.Error(codes.Aborted, "insufficient stock")
		}
		return nil
	})
}

func (s *ProductService) dtmBarrierRestore(ctx context.Context, barrier *dtmcli.BranchBarrier, req *pb.ReduceStockReq) error {
	sqlDB, err := s.repo.DB(ctx).DB()
	if err != nil {
		return apperr.Internal("get sql db", err)
	}
	logID := fmt.Sprintf("restore:%s:%d", req.GetOrderSn(), req.GetSkuId())
	return barrier.CallWithDB(sqlDB, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			"INSERT IGNORE INTO stock_logs (id, order_sn, sku_id, num, action, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			logID, req.GetOrderSn(), req.GetSkuId(), req.GetNum(), "restore", time.Now(),
		)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		_, err = tx.Exec(
			"UPDATE product_skus SET stock = stock + ? WHERE id = ? AND deleted_at IS NULL",
			req.GetNum(), req.GetSkuId(),
		)
		return err
	})
}

func toProductDetail(spu *model.ProductSpu) *pb.GetProductDetailReply {
	reply := &pb.GetProductDetailReply{
		Id:          uint32(spu.ID),
		Name:        spu.Name,
		Description: spu.Description,
		MainImage:   spu.MainImage,
		CategoryId:  uint32(spu.CategoryID),
		BrandId:     uint32(spu.BrandID),
		IsOnSale:    spu.IsOnSale,
	}
	for _, sku := range spu.Skus {
		reply.Skus = append(reply.Skus, &pb.SkuInfo{
			Id:        uint32(sku.ID),
			SkuCode:   sku.SkuCode,
			AttrValue: sku.AttrValue,
			Price:     sku.Price,
			Stock:     int32(sku.Stock),
		})
	}
	return reply
}

func toProductSummary(spu *model.ProductSpu) *pb.ProductSummary {
	summary := &pb.ProductSummary{
		Id:          uint32(spu.ID),
		Name:        spu.Name,
		Description: spu.Description,
		MainImage:   spu.MainImage,
		CategoryId:  uint32(spu.CategoryID),
		BrandId:     uint32(spu.BrandID),
		IsOnSale:    spu.IsOnSale,
	}
	var minPrice float64
	for i, sku := range spu.Skus {
		if i == 0 || sku.Price < minPrice {
			minPrice = sku.Price
		}
		summary.TotalStock += sku.Stock
	}
	summary.MinPrice = minPrice
	return summary
}
