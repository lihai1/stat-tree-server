package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	lotterytree "github.com/lihai1/stat-tree-server/internal/lottery-tree"
	"github.com/lihai1/stat-tree-server/internal/repository"
	"github.com/lihai1/stat-tree-server/internal/utils"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

const version = "1.0.0"

// errUnsupportedGroupSize is returned by GetStatistics when the requested
// group size exceeds the maximum depth the cached tree is built to. The tree
// is built through depth 6 (the six regular numbers drawn), so groups larger
// than six have no meaning and cannot be answered from the cache.
var errUnsupportedGroupSize = errors.New("statistics: form_type must be between 1 and 6")

// LotteryService implements the lotteryv1.LotteryServiceServer interface.
// It is a thin layer over the LotteryManager cache and the LotteryArchive /
// FormGenerator domain objects: each RPC resolves the archive for its date
// window from the cache (building it lazily on the first request) and then
// runs a read-only query or a request-scoped generation against it.
type LotteryService struct {
	lotteryv1.UnimplementedLotteryServiceServer
	manager *LotteryManager
}

// NewLotteryService creates a new LotteryService wired to the lottery manager.
// The manager owns the LRU archive cache and the repository.
func NewLotteryService(manager *LotteryManager) *LotteryService {
	return &LotteryService{manager: manager}
}

// NewLotteryServiceWithRepo is a convenience constructor that wraps a bare
// repository in a fresh manager. It exists for tests and legacy wiring that
// do not share a manager across services.
func NewLotteryServiceWithRepo(repo *repository.LotteryResultRepository) *LotteryService {
	return NewLotteryService(NewLotteryManager(repo))
}

// archive resolves the cached archive for the proto date window, applying the
// default modern-format start date (2004-02-12) when the window has no lower
// bound. A failure to load the archive is returned as an error rather than
// swallowed, so callers never proceed against a nil/invalid archive.
func (s *LotteryService) archive(ctx context.Context, w *lotteryv1.DateWindow) (*lotterytree.LotteryArchive, error) {
	from, to := windowFromProto(w)
	arch, err := s.manager.Archive(ctx, from, to)
	if err != nil {
		return nil, err
	}
	return arch, nil
}

// windowFromProto converts the proto DateWindow into a (from, to) pair of
// time.Time. Zero values mean "open bound" (the manager applies the default
// start date for an open lower bound).
func windowFromProto(w *lotteryv1.DateWindow) (time.Time, time.Time) {
	var from, to time.Time
	if w != nil {
		if ts := w.GetFrom(); ts != nil {
			from = ts.AsTime()
		}
		if ts := w.GetTo(); ts != nil {
			to = ts.AsTime()
		}
	}
	return from, to
}

// HealthCheck returns the health status of the service.
func (s *LotteryService) HealthCheck(ctx context.Context, req *lotteryv1.HealthCheckRequest) (*lotteryv1.HealthCheckResponse, error) {
	drawsLoaded := int32(0)
	// Reach the repository through the manager when present; otherwise the
	// service has no DB and reports zero draws.
	if s.manager != nil && s.manager.repo != nil {
		if count, err := s.manager.repo.Count(ctx); err == nil {
			drawsLoaded = int32(count)
		}
	}
	return &lotteryv1.HealthCheckResponse{
		Status:      "healthy",
		Version:     version,
		DrawsLoaded: drawsLoaded,
	}, nil
}

// GenerateForm generates lottery number combinations based on input parameters.
// It resolves the cached archive for the request window, builds a
// request-scoped FormGenerator, and returns up to howMany non-winning forms
// of formType regular numbers plus the top strong number for the chosen
// strength mode.
func (s *LotteryService) GenerateForm(ctx context.Context, req *lotteryv1.GenerateFormRequest) (*lotteryv1.GenerateFormResponse, error) {
	howMany := int(req.GetHowMany())
	if howMany <= 0 {
		howMany = 1
	}
	if howMany > 50 {
		howMany = 50
	}

	arch, err := s.archive(ctx, req.GetWindow())
	if err != nil {
		slog.Warn("GenerateForm: failed to load archive", "error", err)
		return nil, err
	}

	mode := strengthMode(req.GetStrength())

	formType := int(req.GetFormType())
	if formType < 6 {
		formType = 6
	}

	willBe := utils.Int32sToInts(req.GetWillBe())

	gen := arch.NewFormGenerator(formType, mode)
	results := gen.Generate(howMany, willBe)

	forms := make([]*lotteryv1.NumberSet, 0, len(results))
	for _, row := range results {
		if len(row) == 0 {
			continue
		}
		// Each row is [regular numbers..., strong]. Split the strong off.
		regular := row[:len(row)-1]
		strong := int32(row[len(row)-1])
		ns := &lotteryv1.NumberSet{Numbers: utils.IntsToInt32s(regular)}
		if strong > 0 {
			s := strong
			ns.Strong = &s
		}
		forms = append(forms, ns)
	}

	return &lotteryv1.GenerateFormResponse{Forms: forms}, nil
}

// GetStatistics calculates statistics for number pairs/groups of the
// requested size. The cached tree is built through depth 6, so form_type
// (the group size) must be between 1 and 6; anything larger is rejected.
func (s *LotteryService) GetStatistics(ctx context.Context, req *lotteryv1.GetStatisticsRequest) (*lotteryv1.GetStatisticsResponse, error) {
	groupSize := int(req.GetFormType())
	if groupSize <= 0 {
		groupSize = 2
	}
	if groupSize > 6 {
		return nil, errUnsupportedGroupSize
	}

	arch, err := s.archive(ctx, req.GetWindow())
	if err != nil {
		slog.Warn("GetStatistics: failed to load archive", "error", err)
		return nil, err
	}

	howMany := int(req.GetHowMany())
	if howMany <= 0 {
		howMany = 10
	}

	mode := strengthMode(req.GetStrength())
	entries := arch.TopGroups(howMany, groupSize, mode)

	pairs := make([]*lotteryv1.Pair, 0, len(entries))
	for _, e := range entries {
		pairs = append(pairs, &lotteryv1.Pair{
			Numbers: utils.IntsToInt32s(e.Numbers),
			Count:   int32(e.Count),
		})
	}

	return &lotteryv1.GetStatisticsResponse{Pairs: pairs}, nil
}

// Analyze analyzes user-selected regular numbers against historical data.
// Every number in the form is treated as a regular number; the archive stores
// the strong number separately, so no trailing element is stripped. Forms of
// any length are accepted and every subset up to size 6 is reported.
func (s *LotteryService) Analyze(ctx context.Context, req *lotteryv1.AnalyzeRequest) (*lotteryv1.AnalyzeResponse, error) {
	arch, err := s.archive(ctx, req.GetWindow())
	if err != nil {
		slog.Warn("Analyze: failed to load archive", "error", err)
		return nil, err
	}

	numbers := utils.Int32sToInts(req.GetForm())
	return arch.AnalyzeForm(numbers), nil
}

// strengthMode maps the proto Strength enum to the lottery-tree mode string
// ("strong" / "weak"). The default is "strong".
func strengthMode(st lotteryv1.Strength) string {
	switch st {
	case lotteryv1.Strength_WEAK:
		return string(lotterytree.Weak)
	default:
		return string(lotterytree.Strong)
	}
}
