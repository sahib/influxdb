package replications

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/kit/platform"
	ierrors "github.com/influxdata/influxdb/v2/kit/platform/errors"
	"github.com/influxdata/influxdb/v2/models"
	"github.com/influxdata/influxdb/v2/replications/internal"
	"github.com/influxdata/influxdb/v2/replications/metrics"
	"github.com/influxdata/influxdb/v2/snowflake"
	"github.com/influxdata/influxdb/v2/sqlite"
	"github.com/influxdata/influxdb/v2/storage"
	"github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var errReplicationNotFound = &ierrors.Error{
	Code: ierrors.ENotFound,
	Msg:  "replication not found",
}

func errRemoteNotFound(id platform.ID, cause error) error {
	return &ierrors.Error{
		Code: ierrors.EInvalid,
		Msg:  fmt.Sprintf("remote %q not found", id),
		Err:  cause,
	}
}

func errLocalBucketNotFound(id platform.ID, cause error) error {
	return &ierrors.Error{
		Code: ierrors.EInvalid,
		Msg:  fmt.Sprintf("local bucket %q not found", id),
		Err:  cause,
	}
}

func NewService(store *sqlite.SqlStore, bktSvc BucketService, localWriter storage.PointsWriter, log *zap.Logger, enginePath string) (*service, *metrics.ReplicationsMetrics) {
	metrs := metrics.NewReplicationsMetrics()

	return &service{
		store:         store,
		idGenerator:   snowflake.NewIDGenerator(),
		bucketService: bktSvc,
		localWriter:   localWriter,
		validator:     internal.NewValidator(),
		log:           log,
		durableQueueManager: internal.NewDurableQueueManager(
			log,
			filepath.Join(enginePath, "replicationq"),
			metrs,
		),
	}, metrs
}

type ReplicationValidator interface {
	ValidateReplication(context.Context, *internal.ReplicationHTTPConfig) error
}

type BucketService interface {
	RLock()
	RUnlock()
	FindBucketByID(ctx context.Context, id platform.ID) (*influxdb.Bucket, error)
}

type DurableQueueManager interface {
	InitializeQueue(replicationID platform.ID, maxQueueSizeBytes int64) error
	DeleteQueue(replicationID platform.ID) error
	UpdateMaxQueueSize(replicationID platform.ID, maxQueueSizeBytes int64) error
	CurrentQueueSizes(ids []platform.ID) (map[platform.ID]int64, error)
	StartReplicationQueues(trackedReplications map[platform.ID]int64) error
	CloseAll() error
	EnqueueData(replicationID platform.ID, data []byte, numPoints int) error
}

type service struct {
	store               *sqlite.SqlStore
	idGenerator         platform.IDGenerator
	bucketService       BucketService
	validator           ReplicationValidator
	durableQueueManager DurableQueueManager
	localWriter         storage.PointsWriter
	log                 *zap.Logger
}

func (s service) ListReplications(ctx context.Context, filter influxdb.ReplicationListFilter) (*influxdb.Replications, error) {
	q := sq.Select(
		"id", "org_id", "name", "description", "remote_id", "local_bucket_id", "remote_bucket_id",
		"max_queue_size_bytes", "latest_response_code", "latest_error_message", "drop_non_retryable_data").
		From("replications").
		Where(sq.Eq{"org_id": filter.OrgID})

	if filter.Name != nil {
		q = q.Where(sq.Eq{"name": *filter.Name})
	}
	if filter.RemoteID != nil {
		q = q.Where(sq.Eq{"remote_id": *filter.RemoteID})
	}
	if filter.LocalBucketID != nil {
		q = q.Where(sq.Eq{"local_bucket_id": *filter.LocalBucketID})
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	var rs influxdb.Replications
	if err := s.store.DB.SelectContext(ctx, &rs.Replications, query, args...); err != nil {
		return nil, err
	}

	if len(rs.Replications) == 0 {
		return &rs, nil
	}

	ids := make([]platform.ID, len(rs.Replications))
	for i := range rs.Replications {
		ids[i] = rs.Replications[i].ID
	}
	sizes, err := s.durableQueueManager.CurrentQueueSizes(ids)
	if err != nil {
		return nil, err
	}
	for i := range rs.Replications {
		rs.Replications[i].CurrentQueueSizeBytes = sizes[rs.Replications[i].ID]
	}

	return &rs, nil
}

func (s service) CreateReplication(ctx context.Context, request influxdb.CreateReplicationRequest) (*influxdb.Replication, error) {
	s.bucketService.RLock()
	defer s.bucketService.RUnlock()

	s.store.Mu.Lock()
	defer s.store.Mu.Unlock()

	if _, err := s.bucketService.FindBucketByID(ctx, request.LocalBucketID); err != nil {
		return nil, errLocalBucketNotFound(request.LocalBucketID, err)
	}

	newID := s.idGenerator.ID()
	if err := s.durableQueueManager.InitializeQueue(newID, request.MaxQueueSizeBytes); err != nil {
		return nil, err
	}

	q := sq.Insert("replications").
		SetMap(sq.Eq{
			"id":                      newID,
			"org_id":                  request.OrgID,
			"name":                    request.Name,
			"description":             request.Description,
			"remote_id":               request.RemoteID,
			"local_bucket_id":         request.LocalBucketID,
			"remote_bucket_id":        request.RemoteBucketID,
			"max_queue_size_bytes":    request.MaxQueueSizeBytes,
			"drop_non_retryable_data": request.DropNonRetryableData,
			"created_at":              "datetime('now')",
			"updated_at":              "datetime('now')",
		}).
		Suffix("RETURNING id, org_id, name, description, remote_id, local_bucket_id, remote_bucket_id, max_queue_size_bytes, drop_non_retryable_data")

	cleanupQueue := func() {
		if cleanupErr := s.durableQueueManager.DeleteQueue(newID); cleanupErr != nil {
			s.log.Warn("durable queue remaining on disk after initialization failure", zap.Error(cleanupErr), zap.String("id", newID.String()))
		}
	}

	query, args, err := q.ToSql()
	if err != nil {
		cleanupQueue()
		return nil, err
	}

	var r influxdb.Replication

	if err := s.store.DB.GetContext(ctx, &r, query, args...); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && sqlErr.ExtendedCode == sqlite3.ErrConstraintForeignKey {
			cleanupQueue()
			return nil, errRemoteNotFound(request.RemoteID, err)
		}
		cleanupQueue()
		return nil, err
	}

	return &r, nil
}

func (s service) ValidateNewReplication(ctx context.Context, request influxdb.CreateReplicationRequest) error {
	if _, err := s.bucketService.FindBucketByID(ctx, request.LocalBucketID); err != nil {
		return errLocalBucketNotFound(request.LocalBucketID, err)
	}

	config := internal.ReplicationHTTPConfig{RemoteBucketID: request.RemoteBucketID}
	if err := s.populateRemoteHTTPConfig(ctx, request.RemoteID, &config); err != nil {
		return err
	}

	if err := s.validator.ValidateReplication(ctx, &config); err != nil {
		return &ierrors.Error{
			Code: ierrors.EInvalid,
			Msg:  "replication parameters fail validation",
			Err:  err,
		}
	}
	return nil
}

func (s service) GetReplication(ctx context.Context, id platform.ID) (*influxdb.Replication, error) {
	q := sq.Select(
		"id", "org_id", "name", "description", "remote_id", "local_bucket_id", "remote_bucket_id",
		"max_queue_size_bytes", "latest_response_code", "latest_error_message", "drop_non_retryable_data").
		From("replications").
		Where(sq.Eq{"id": id})

	query, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	var r influxdb.Replication
	if err := s.store.DB.GetContext(ctx, &r, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errReplicationNotFound
		}
		return nil, err
	}

	sizes, err := s.durableQueueManager.CurrentQueueSizes([]platform.ID{r.ID})
	if err != nil {
		return nil, err
	}
	r.CurrentQueueSizeBytes = sizes[r.ID]

	return &r, nil
}

func (s service) UpdateReplication(ctx context.Context, id platform.ID, request influxdb.UpdateReplicationRequest) (*influxdb.Replication, error) {
	s.store.Mu.Lock()
	defer s.store.Mu.Unlock()

	updates := sq.Eq{"updated_at": sq.Expr("datetime('now')")}
	if request.Name != nil {
		updates["name"] = *request.Name
	}
	if request.Description != nil {
		updates["description"] = *request.Description
	}
	if request.RemoteID != nil {
		updates["remote_id"] = *request.RemoteID
	}
	if request.RemoteBucketID != nil {
		updates["remote_bucket_id"] = *request.RemoteBucketID
	}
	if request.MaxQueueSizeBytes != nil {
		updates["max_queue_size_bytes"] = *request.MaxQueueSizeBytes
	}
	if request.DropNonRetryableData != nil {
		updates["drop_non_retryable_data"] = *request.DropNonRetryableData
	}

	q := sq.Update("replications").SetMap(updates).Where(sq.Eq{"id": id}).
		Suffix("RETURNING id, org_id, name, description, remote_id, local_bucket_id, remote_bucket_id, max_queue_size_bytes, drop_non_retryable_data")

	query, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	var r influxdb.Replication
	if err := s.store.DB.GetContext(ctx, &r, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errReplicationNotFound
		}
		if sqlErr, ok := err.(sqlite3.Error); ok && request.RemoteID != nil && sqlErr.ExtendedCode == sqlite3.ErrConstraintForeignKey {
			return nil, errRemoteNotFound(*request.RemoteID, err)
		}
		return nil, err
	}

	if request.MaxQueueSizeBytes != nil {
		if err := s.durableQueueManager.UpdateMaxQueueSize(id, *request.MaxQueueSizeBytes); err != nil {
			s.log.Warn("actual max queue size does not match the max queue size recorded in database", zap.String("id", id.String()))
			return nil, err
		}
	}

	sizes, err := s.durableQueueManager.CurrentQueueSizes([]platform.ID{r.ID})
	if err != nil {
		return nil, err
	}
	r.CurrentQueueSizeBytes = sizes[r.ID]

	return &r, nil
}

func (s service) ValidateUpdatedReplication(ctx context.Context, id platform.ID, request influxdb.UpdateReplicationRequest) error {
	baseConfig, err := s.getFullHTTPConfig(ctx, id)
	if err != nil {
		return err
	}
	if request.RemoteBucketID != nil {
		baseConfig.RemoteBucketID = *request.RemoteBucketID
	}

	if request.RemoteID != nil {
		if err := s.populateRemoteHTTPConfig(ctx, *request.RemoteID, baseConfig); err != nil {
			return err
		}
	}

	if err := s.validator.ValidateReplication(ctx, baseConfig); err != nil {
		return &ierrors.Error{
			Code: ierrors.EInvalid,
			Msg:  "validation fails after applying update",
			Err:  err,
		}
	}
	return nil
}

func (s service) DeleteReplication(ctx context.Context, id platform.ID) error {
	s.store.Mu.Lock()
	defer s.store.Mu.Unlock()

	q := sq.Delete("replications").Where(sq.Eq{"id": id}).Suffix("RETURNING id")
	query, args, err := q.ToSql()
	if err != nil {
		return err
	}

	var d platform.ID
	if err := s.store.DB.GetContext(ctx, &d, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errReplicationNotFound
		}
		return err
	}

	if err := s.durableQueueManager.DeleteQueue(id); err != nil {
		return err
	}

	return nil
}

func (s service) DeleteBucketReplications(ctx context.Context, localBucketID platform.ID) error {
	s.store.Mu.Lock()
	defer s.store.Mu.Unlock()

	q := sq.Delete("replications").Where(sq.Eq{"local_bucket_id": localBucketID}).Suffix("RETURNING id")
	query, args, err := q.ToSql()
	if err != nil {
		return err
	}

	var deleted []string
	if err := s.store.DB.SelectContext(ctx, &deleted, query, args...); err != nil {
		return err
	}

	errOccurred := false
	for _, replication := range deleted {
		id, err := platform.IDFromString(replication)
		if err != nil {
			s.log.Error("durable queue remaining on disk after deletion failure", zap.Error(err), zap.String("id", replication))
			errOccurred = true
		}

		if err := s.durableQueueManager.DeleteQueue(*id); err != nil {
			s.log.Error("durable queue remaining on disk after deletion failure", zap.Error(err), zap.String("id", replication))
			errOccurred = true
		}
	}

	s.log.Debug("Deleted all replications for local bucket",
		zap.String("bucket_id", localBucketID.String()), zap.Strings("ids", deleted))

	if errOccurred {
		return fmt.Errorf("deleting replications for bucket %q failed, see server logs for details", localBucketID)
	}

	return nil
}

func (s service) ValidateReplication(ctx context.Context, id platform.ID) error {
	config, err := s.getFullHTTPConfig(ctx, id)
	if err != nil {
		return err
	}
	if err := s.validator.ValidateReplication(ctx, config); err != nil {
		return &ierrors.Error{
			Code: ierrors.EInvalid,
			Msg:  "replication failed validation",
			Err:  err,
		}
	}
	return nil
}

func (s service) WritePoints(ctx context.Context, orgID platform.ID, bucketID platform.ID, points []models.Point) error {
	q := sq.Select("id").From("replications").Where(sq.Eq{"org_id": orgID, "local_bucket_id": bucketID})
	query, args, err := q.ToSql()
	if err != nil {
		return err
	}

	var ids []platform.ID
	if err := s.store.DB.SelectContext(ctx, &ids, query, args...); err != nil {
		return err
	}

	// If there are no registered replications, all we need to do is a local write.
	if len(ids) == 0 {
		return s.localWriter.WritePoints(ctx, orgID, bucketID, points)
	}

	// Concurrently...
	var egroup errgroup.Group
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)

	// 1. Write points to local TSM
	egroup.Go(func() error {
		return s.localWriter.WritePoints(ctx, orgID, bucketID, points)
	})
	// 2. Serialize points to gzipped line protocol, to be enqueued for replication if the local write succeeds.
	//    We gzip the LP to take up less room on disk. On the other end of the queue, we can send the gzip data
	//    directly to the remote API without needing to decompress it.
	egroup.Go(func() error {
		for _, p := range points {
			if _, err := gzw.Write(append([]byte(p.PrecisionString("ns")), '\n')); err != nil {
				_ = gzw.Close()
				return fmt.Errorf("failed to serialize points for replication: %w", err)
			}
		}
		if err := gzw.Close(); err != nil {
			return err
		}
		return nil
	})

	if err := egroup.Wait(); err != nil {
		return err
	}

	// Enqueue the data into all registered replications.
	var wg sync.WaitGroup
	wg.Add(len(ids))
	for _, id := range ids {
		go func(id platform.ID) {
			defer wg.Done()
			if err := s.durableQueueManager.EnqueueData(id, buf.Bytes(), len(points)); err != nil {
				s.log.Error("Failed to enqueue points for replication", zap.String("id", id.String()), zap.Error(err))
			}
		}(id)
	}
	wg.Wait()

	return nil
}

func (s service) getFullHTTPConfig(ctx context.Context, id platform.ID) (*internal.ReplicationHTTPConfig, error) {
	q := sq.Select("c.remote_url", "c.remote_api_token", "c.remote_org_id", "c.allow_insecure_tls", "r.remote_bucket_id").
		From("replications r").InnerJoin("remotes c ON r.remote_id = c.id AND r.id = ?", id)

	query, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	var rc internal.ReplicationHTTPConfig
	if err := s.store.DB.GetContext(ctx, &rc, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errReplicationNotFound
		}
		return nil, err
	}
	return &rc, nil
}

func (s service) populateRemoteHTTPConfig(ctx context.Context, id platform.ID, target *internal.ReplicationHTTPConfig) error {
	q := sq.Select("remote_url", "remote_api_token", "remote_org_id", "allow_insecure_tls").
		From("remotes").Where(sq.Eq{"id": id})
	query, args, err := q.ToSql()
	if err != nil {
		return err
	}

	if err := s.store.DB.GetContext(ctx, target, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRemoteNotFound(id, nil)
		}
		return err
	}

	return nil
}

func (s service) Open(ctx context.Context) error {
	var trackedReplications influxdb.Replications

	// Get replications from sqlite
	q := sq.Select(
		"id", "max_queue_size_bytes").
		From("replications")

	query, args, err := q.ToSql()
	if err != nil {
		return err
	}

	if err := s.store.DB.SelectContext(ctx, &trackedReplications.Replications, query, args...); err != nil {
		return err
	}

	trackedReplicationsMap := make(map[platform.ID]int64)
	for _, r := range trackedReplications.Replications {
		trackedReplicationsMap[r.ID] = r.MaxQueueSizeBytes
	}

	// Queue manager completes startup tasks
	if err := s.durableQueueManager.StartReplicationQueues(trackedReplicationsMap); err != nil {
		return err
	}
	return nil
}

func (s service) Close() error {
	if err := s.durableQueueManager.CloseAll(); err != nil {
		return err
	}
	return nil
}
