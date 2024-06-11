# DataMesh数据读写
Kuscia DataMesh 提供了基于gRpc的数据读写操作，实现了Arrow Flight 提供的标准的数据服务接口.你可以通过初始化一个flight client 来发起数据的请求。
## 如何通过Arrow Flight访问DataMesh
FlightServiceClient 是 Apache Arrow Flight 框架中客户端的主要接口，它提供了一系列的方法来与 Flight 服务交互，例如获取数据集信息、上传和下载数据等。在本例中，使用 Go 语言展示如何使用 flight.FlightServiceClient。

要使用 FlightServiceClient，你需要先建立一个与 Flight 服务端的 gRPC 连接，然后通过这个连接创建一个 FlightServiceClient 实例。
### 查询数据
![Alt text](../../../imgs/flight_do_get.png)
### 上传数据
![Alt text](../../../imgs/flight_do_put.png)
## DataMesh 支持的数据服务
DataMesh 当前仅支持以下查询能力:
```go
type FlightServiceClient interface {
	GetFlightInfo(ctx context.Context, in *FlightDescriptor, opts ...grpc.CallOption) (*FlightInfo, error)
	// For a given FlightDescriptor, get the Schema as described in Schema.fbs::Schema
	// This is used when a consumer needs the Schema of flight stream. Similar to
	// GetFlightInfo this interface may generate a new flight that was not previously
	// available in ListFlights.
	GetSchema(ctx context.Context, in *FlightDescriptor, opts ...grpc.CallOption) (*SchemaResult, error)
	// Retrieve a single stream associated with a particular descriptor
	// associated with the referenced ticket. A Flight can be composed of one or
	// more streams where each stream can be retrieved using a separate opaque
	// ticket that the flight service uses for managing a collection of streams.
	DoGet(ctx context.Context, in *Ticket, opts ...grpc.CallOption) (FlightService_DoGetClient, error)
	// Push a stream to the flight service associated with a particular
	// flight stream. This allows a client of a flight service to upload a stream
	// of data. Depending on the particular flight service, a client consumer
	// could be allowed to upload a single stream per descriptor or an unlimited
	// number. In the latter, the service might implement a 'seal' action that
	// can be applied to a descriptor once all streams are uploaded.
	DoPut(ctx context.Context, opts ...grpc.CallOption) (FlightService_DoPutClient, error)
	// Flight services can support an arbitrary number of simple actions in
	// addition to the possible ListFlights, GetFlightInfo, DoGet, DoPut
	// operations that are potentially available. DoAction allows a flight client
	// to do a specific action against a flight service. An action includes
	// opaque request and response objects that are specific to the type action
	// being undertaken.
	DoAction(ctx context.Context, in *Action, opts ...grpc.CallOption) (FlightService_DoActionClient, error)
}
```