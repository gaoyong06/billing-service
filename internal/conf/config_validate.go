package conf

import "fmt"

// Validate validates the configuration
func (b *Bootstrap) Validate() error {
	if b.Server == nil {
		return fmt.Errorf("server configuration is required")
	}
	if b.Server.Http == nil || b.Server.Http.Addr == "" {
		return fmt.Errorf("server.http.addr is required")
	}
	if b.Server.Grpc == nil || b.Server.Grpc.Addr == "" {
		return fmt.Errorf("server.grpc.addr is required")
	}
	if b.Data == nil {
		return fmt.Errorf("data configuration is required")
	}
	if b.Data.Database == nil || b.Data.Database.Source == "" {
		return fmt.Errorf("data.database.source is required")
	}
	return nil
}
