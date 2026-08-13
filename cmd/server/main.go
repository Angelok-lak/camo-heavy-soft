// The backend is ONE Go binary (architecture decision: no serverless —
// the database runs around the clock anyway). App Runner or Fargate run
// this same container; a future multi-school SaaS runs more instances of
// it, nothing gets rewritten (D-01).
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/angelok-lak/camo-heavy-soft/internal/attendance"
	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
	"github.com/angelok-lak/camo-heavy-soft/internal/comms"
	"github.com/angelok-lak/camo-heavy-soft/internal/docs"
	"github.com/angelok-lak/camo-heavy-soft/internal/enrollment"
	"github.com/angelok-lak/camo-heavy-soft/internal/exams"
	"github.com/angelok-lak/camo-heavy-soft/internal/people"
	"github.com/angelok-lak/camo-heavy-soft/internal/planning"
	"github.com/angelok-lak/camo-heavy-soft/internal/resources"
	"github.com/angelok-lak/camo-heavy-soft/internal/settings"
	"github.com/angelok-lak/camo-heavy-soft/internal/tasks"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	authSvc := auth.NewService(pool)
	authH := auth.NewHandler(authSvc)

	api := http.NewServeMux()
	planningH := planning.NewHandler(planning.NewStore(pool))
	planningH.Routes(api)
	settings.NewHandler(settings.NewStore(pool)).Routes(api)
	resources.NewHandler(pool).Routes(api)
	people.NewHandler(pool).Routes(api)
	enrollment.NewHandler(pool).Routes(api)
	attendance.NewHandler(pool).Routes(api)
	exams.NewHandler(pool).Routes(api)
	tasks.NewHandler(pool).Routes(api)
	commsH := comms.NewHandler(pool)
	commsH.Routes(api)
	planningH.NotifyAssignment = func(ctx context.Context, schoolID, lessonID, enrollmentID, authorID uuid.UUID) (any, error) {
		c, err := commsH.NotifyLessonAssignment(ctx, schoolID, lessonID, enrollmentID, authorID)
		if c == nil {
			return nil, err
		}
		return c, err
	}
	docsH := docs.NewHandler(pool)
	docsH.Routes(api)
	authH.PrivateRoutes(api)

	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	authH.PublicRoutes(root)
	docsH.PublicRoutes(root)
	root.Handle("/api/", auth.Middleware(authSvc, api))

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, root))
}
