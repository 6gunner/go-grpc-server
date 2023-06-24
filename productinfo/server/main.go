package main

import (
	"context"
	"fmt"
	"github.com/gofrs/uuid"
	wrapper "github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"io"
	"log"
	"net"
	om "productinfo/common"
	pb "productinfo/server/ecommerce"
	"strings"
)

const (
	port           = ":50051"
	orderBatchSize = 3
)

var orderMap = make(map[string]om.Order) // 值类型为 om.Order 结构体

type server struct {
	// 用来存储grpc server的引用
	productMap map[string]*pb.Product
	orderMap   map[string]*om.Order // 值类型为指向 om.Order的结构体指针
}

// 在server上实现 ecommerce.GetProduct
func (s *server) GetProduct(ctx context.Context, id *pb.ProductID) (*pb.Product, error) {

	product, exists := s.productMap[id.Value]

	if exists && product != nil {
		log.Printf("查询产品成功，id=%v 对应的productName=%v", product.Id, product.Name)
		return product, status.New(codes.OK, "").Err()
	}
	return nil, status.Errorf(codes.NotFound, "产品未找到", id.Value)
}

// 在server上实现 ecommerce.AddProduct
func (s *server) AddProduct(ctx context.Context, product *pb.Product) (*pb.ProductID, error) {
	out, err := uuid.NewV4()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error while generating Product ID", err)

	}
	product.Id = out.String()
	if s.productMap == nil {
		s.productMap = make(map[string]*pb.Product)
	}
	s.productMap[product.Id] = product
	log.Printf("添加产品成功, product id=%v, productName=%v", product.Id, product.Name)
	return &pb.ProductID{Value: product.Id}, status.New(codes.OK, "").Err()
}

func (s *server) GetOrder(ctx context.Context, orderId *wrapper.StringValue) (*om.Order, error) {
	log.Printf("查询订单, order id = %v", orderId)

	ord, exists := orderMap[orderId.Value]

	if exists {
		return &ord, status.New(codes.OK, "").Err()
	}
	return nil, status.Errorf(codes.NotFound, "订单未找到", orderId.Value)

}

/**
 *	搜索订单
 *  @param searchValue 要搜索的字符串
 *  @param orderServerStream 流的引用对象，可以写入多个响应
 */
func (s *server) SearchOrder(searchValue *wrapperspb.StringValue, orderServerStream om.OrderManagement_SearchOrderServer) error {
	log.Printf("SearchOrder Order 【searcValue:%v】", searchValue)
	for key, order := range orderMap {
		log.Printf(key, order)
		for _, itemStr := range order.Items {
			log.Printf(itemStr)
			if strings.Contains(itemStr, searchValue.Value) {
				err := orderServerStream.Send(&order)
				if err != nil {
					return fmt.Errorf("error sending message to stream %v", err)
				}
				log.Printf("Matching Order Found :" + key)
				break
			}
		}
	}
	return nil
}

/**
 * 更新订单
 */
func (s *server) UpdateOrder(orderServerStream om.OrderManagement_UpdateOrderServer) error {
	ordersStr := "Updated Order IDs :"
	for {
		// 从客户端读取消息
		order, err := orderServerStream.Recv()
		if err == io.EOF {
			return orderServerStream.SendAndClose(&wrapper.StringValue{
				Value: "订单处理完成，" + ordersStr,
			})
		}
		// 更新订单
		orderMap[order.Id] = *order

		log.Printf("Order ID ", order.Id, " 更新完成")
		ordersStr += order.Id + ","
	}

}

/*
* 处理订单，打包发货
双向rpc通道处理订单
*/
func (s *server) ProcessOrder(orderServerStream om.OrderManagement_ProcessOrderServer) error {
	//TODO implement me
	batchMarker := 1
	// 创建一个map对象，映射对应的orderId和CombinedShipment
	var combinedShipmentMap = make(map[string]om.CombinedShipment)
	for {
		orderId, err := orderServerStream.Recv()
		log.Printf("读取ProcessOrder的消息 : %s", orderId)
		if err == io.EOF { // 流结束了
			log.Printf("EOF : %s", orderId)
			for _, shipment := range combinedShipmentMap {
				if err := orderServerStream.Send(&shipment); err != nil {
					return err
				}
			}
			return nil
		}
		if err != nil {
			log.Println(err)
			return err
		}

		destination := orderMap[orderId.GetValue()].Destination
		shipment, found := combinedShipmentMap[destination]

		// 有同一个目的地的
		if found {
			ord := orderMap[orderId.GetValue()]
			shipment.OrderList = append(shipment.OrderList, &ord)
			combinedShipmentMap[destination] = shipment
		} else {
			ord := orderMap[orderId.GetValue()]
			comShip := om.CombinedShipment{
				Id:     "cmb-" + (ord.Destination),
				Status: "Processed!",
			}
			comShip.OrderList = append(shipment.OrderList, &ord)
			combinedShipmentMap[destination] = comShip
		}

		if batchMarker == orderBatchSize {
			for _, comb := range combinedShipmentMap {
				log.Printf("运送 : %v -> %v", comb.Id, len(comb.OrderList))
				if err := orderServerStream.Send(&comb); err != nil {
					return err
				}
			}
			batchMarker = 0
			combinedShipmentMap = make(map[string]om.CombinedShipment)
		} else {
			batchMarker++
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
	// 创建一个gRpc服务实例
	s := grpc.NewServer()
	// &server{}是什么意思？ 构造一个server结果体
	// 将我们的服务（&server{}）注册到gRpc服务实例（s）上；
	pb.RegisterProductInfoServer(s, &server{})
	om.RegisterOrderManagementServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("服务启动失败, %v", err)
	}
}
