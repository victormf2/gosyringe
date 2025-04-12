package setup

import (
	"database/sql"

	"github.com/victormf2/gosyringe"
	"github.com/victormf2/gosyringe/internal/examples/gin/handlers"
	"github.com/victormf2/gosyringe/internal/examples/gin/infra"
	"github.com/victormf2/gosyringe/internal/examples/gin/repositories"
)

func RegisterServices(c *gosyringe.Container) {

	gosyringe.RegisterValue(c, infra.DBConfig{
		Path: "./database.db",
	})
	gosyringe.RegisterSingleton[*sql.DB](c, infra.NewDB)
	gosyringe.RegisterScoped[repositories.IUserRepository](c, repositories.NewUserRepository)
	gosyringe.RegisterScoped[*handlers.GetUserByIDHandler](c, handlers.NewGetUserByIDHandler)
	gosyringe.RegisterScoped[*handlers.CreateUserHandler](c, handlers.NewCreateUserHandler)
	gosyringe.RegisterScoped[*handlers.UpdateUserHandler](c, handlers.NewUpdateUserHandler)
	gosyringe.RegisterScoped[*handlers.DeleteUserHandler](c, handlers.NewDeleteUserHandler)
}
