package main

import (
	"context"
	wrapper "github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc"
	"io"
	"log"
	pb "productinfo/client/ecommerce"
	om "productinfo/common"
	"time"
)

const (
	address = "localhost:50051"
)

func main() {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("服务连接失败: %v", err)
	}
	// defer会做延迟
	defer conn.Close()
	// 建立连接1
	client1 := pb.NewProductInfoClient(conn)
	name := "Apple iPhone 11"
	description := "Meet Apple iPhone 11. All-new dual-camera system"
	price := float32(1000.0)

	// 建立连接2
	client2 := om.NewOrderManagementClient(conn)

	// 创建context对象
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// 添加商品
	result, err := client1.AddProduct(ctx, &pb.Product{Name: name, Description: description, Price: price})
	if err != nil {
		log.Fatalf("添加商品失败 %v", err)
	}
	log.Printf("Product ID: %s 添加成功", result.Value)

	// 获取商品1
	//product, err := client1.GetProduct(ctx, &pb.ProductID{Value: result.Value})
	//if err != nil {
	//	log.Fatalf("无法找到对应的产品 %v", err)
	//}
	//log.Printf("Product: %v", product.String())

	// 获取商品2
	//product2, err := client1.GetProduct(ctx, &pb.ProductID{Value: "mock"})
	//if err != nil {
	//	log.Fatalf("无法找到对应的产品 %v", err)
	//}
	//log.Printf("Product: %v", product2.String())

	// 获取订单
	//retrievedOrder, err := client2.GetOrder(ctx, &wrapper.StringValue{Value: "106"})
	//if err != nil {
	//	log.Fatalf("无法找到对应的订单 %v", err)
	//}
	//log.Printf("GetOrder Response ->:", retrievedOrder)

	updateOrderStream, err := client2.UpdateOrder(ctx)
	if err != nil {
		log.Fatalf("UpdateOrder Error %v", err)
	}
	// 添加订单1
	updateOrder1 := om.Order{Id: "102", Items: []string{"Google Pixel 3A", "Google Pixel Book"}, Destination: "Mountain View, CA", Price: 1100.00}
	if err := updateOrderStream.Send(&updateOrder1); err != nil {
		log.Fatalf("%v.Send(%v) = %v", updateOrderStream, updateOrder1, err)
	}

	// 添加订单2
	updateOrder2 := om.Order{Id: "103", Items: []string{"Apple Watch S4", "Mac Book Pro", "iPad Pro"}, Destination: "San Jose, CA", Price: 2800.00}
	if err := updateOrderStream.Send(&updateOrder2); err != nil {
		log.Fatalf("%v.Send(%v) = %v", updateOrderStream, updateOrder2, err)
	}
	// 添加订单3
	updateOrder3 := om.Order{Id: "104", Items: []string{"Google Home Mini", "Google Nest Hub", "iPad Mini"}, Destination: "Mountain View, CA", Price: 2200.00}
	if err := updateOrderStream.Send(&updateOrder3); err != nil {
		log.Fatalf("%v.Send(%v) = %v", updateOrderStream, updateOrder3, err)
	}
	// 添加订单4
	updateOrder4 := om.Order{Id: "105", Items: []string{"Amazon Echo"}, Destination: "San Jose, CA", Price: 30.00}
	if err := updateOrderStream.Send(&updateOrder4); err != nil {
		log.Fatalf("%v.Send(%v) = %v", updateOrderStream, updateOrder4, err)
	}

	// 添加订单5
	updateOrder5 := om.Order{Id: "106", Items: []string{"Amazon Echo", "Apple iPhone XS"}, Destination: "Mountain View, CA", Price: 300.00}
	if err := updateOrderStream.Send(&updateOrder5); err != nil {
		log.Fatalf("%v.Send(%v) = %v", updateOrderStream, updateOrder5, err)
	}

	updateRes, err := updateOrderStream.CloseAndRecv()
	if err != nil {
		log.Fatalf("%v.CloseAndRecv() got error %v", updateOrderStream, err)
	}
	log.Printf("UpdateOrder resp : %s", updateRes)
	// 搜索订单
	//searchStream, err := client2.SearchOrder(ctx, &wrapper.StringValue{Value: "Google"})
	//if err != nil {
	//	log.Fatalf("SearchOrder Error %v", err)
	//}
	//if searchStream == nil {
	//	log.Printf("搜索结果为空")
	//}
	//for {
	//	searchOrder, err := searchStream.Recv()
	//	// 如果流结束了
	//	if err != nil {
	//		if err == io.EOF {
	//			break
	//		}
	//		log.Fatalf("搜索出错了, error = %v", err)
	//	}
	//	log.Printf("搜索结果 : ", searchOrder)
	//	// todo 放到一个list里去
	//}

	// 拿到streamProcOrder
	streamProcOrder, err := client2.ProcessOrder(ctx)
	if err != nil {
		log.Fatalf("%v.ProcessOrders(_) = _, %v", client2, err)
	}
	if err := streamProcOrder.Send(&wrapper.StringValue{Value: "102"}); err != nil {
		log.Fatalf("%v.Send(%v) = %v", client2, "102", err)
	}
	if err := streamProcOrder.Send(&wrapper.StringValue{Value: "103"}); err != nil {
		log.Fatalf("%v.Send(%v) = %v", client2, "103", err)
	}
	if err := streamProcOrder.Send(&wrapper.StringValue{Value: "104"}); err != nil {
		log.Fatalf("%v.Send(%v) = %v", client2, "104", err)
	}
	channel := make(chan struct{}) // 创建了一个没有缓冲的通道，用于在协程之间进行同步和通信
	go asyncClientBidirectionalRPC(streamProcOrder, channel)

	if err := streamProcOrder.Send(&wrapper.StringValue{Value: "101"}); err != nil {
		log.Fatalf("%v.Send(%v) = %v", client2, "101", err)
	}
	if err := streamProcOrder.CloseSend(); err != nil {
		log.Fatal(err)
	}
	<-channel // 阻塞代码，等待从通道中接收到一个值
}

// 开启一个goRouting协程，来处理服务端流rpc发送的消息
func asyncClientBidirectionalRPC(streamProcOrder om.OrderManagement_ProcessOrderClient, channel chan struct{}) {

	for {
		combinedShipment, errProcOrder := streamProcOrder.Recv()
		if errProcOrder == io.EOF {
			break
		}
		log.Printf("打包运输 : ", combinedShipment.OrderList)
	}
	<-channel
}
