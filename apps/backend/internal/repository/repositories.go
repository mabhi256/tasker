package repository

import (
	"github.com/mabhi256/tasker/internal/server"
)

type Repositories struct {
	Todo     *TodoRepository
	Category *CategoryRepository
	Comment  *CommentRepository
}

func NewRepositories(s *server.Server) *Repositories {
	return &Repositories{
		Todo:     NewTodoRepository(s),
		Category: NewCategoryRepository(s),
		Comment:  NewCommentRepository(s),
	}
}
