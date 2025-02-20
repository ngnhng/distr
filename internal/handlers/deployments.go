package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/glasskube/distr/api"
	"github.com/glasskube/distr/internal/apierrors"
	"github.com/glasskube/distr/internal/auth"
	internalctx "github.com/glasskube/distr/internal/context"
	"github.com/glasskube/distr/internal/db"
	"github.com/glasskube/distr/internal/middleware"
	"github.com/glasskube/distr/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func DeploymentsRouter(r chi.Router) {
	r.Use(middleware.RequireOrgID, middleware.RequireUserRole)
	r.Put("/", putDeployment)
	r.Route("/{deploymentId}", func(r chi.Router) {
		r.Use(deploymentMiddleware)
		r.Get("/status", getDeploymentStatus)
	})
}

func putDeployment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := internalctx.GetLogger(ctx)
	auth := auth.Authentication.Require(ctx)
	orgId := auth.CurrentOrgID()

	deploymentRequest, err := JsonBody[api.DeploymentRequest](w, r)
	if err != nil {
		return
	}

	_ = db.RunTx(ctx, pgx.TxOptions{}, func(ctx context.Context) error {
		if application, err := db.GetApplicationForApplicationVersionID(
			ctx, deploymentRequest.ApplicationVersionID, *auth.CurrentOrgID(),
		); errors.Is(err, apierrors.ErrNotFound) {
			http.Error(w, "application does not exist", http.StatusBadRequest)
			return err
		} else if err != nil {
			log.Warn("could not get application version", zap.Error(err))
			http.Error(w, "an internal error occurred", http.StatusInternalServerError)
			return err
		} else if appVersion, err := db.GetApplicationVersion(ctx, deploymentRequest.ApplicationVersionID); err != nil {
			http.Error(w, "application version does not exist", http.StatusBadRequest)
			return err
		} else if appVersionValues, err := appVersion.ParsedValuesFile(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		} else if deploymentTarget, err := db.GetDeploymentTarget(
			ctx, deploymentRequest.DeploymentTargetID, orgId,
		); errors.Is(err, apierrors.ErrNotFound) {
			http.Error(w, "deployment target does not exist", http.StatusBadRequest)
			return err
		} else if err != nil {
			log.Warn("could not get deployment target", zap.Error(err))
			http.Error(w, "an inernal error occurred", http.StatusInternalServerError)
			return err
		} else if deploymentTarget.Type != application.Type {
			msg := "application and deployment target must have the same type"
			http.Error(w, msg, http.StatusBadRequest)
			return errors.New(msg)
		} else if deploymentValues, err := deploymentRequest.ParsedValuesFile(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		} else if _, err := util.MergeAllRecursive(appVersionValues, deploymentValues); err != nil {
			http.Error(w, fmt.Sprintf("values cannot be merged with base: %v", err), http.StatusBadRequest)
			return err
		} else {
			if IsEmptyUUID(deploymentRequest.ID) {
				if deploymentTarget.Deployment != nil {
					msg := "only one deployment per target is supported right now"
					http.Error(w, msg, http.StatusBadRequest)
					return errors.New(msg)
				}
			} else if deploymentTarget.Deployment == nil {
				msg := "given deployment is not a deployment of the given target"
				http.Error(w, msg, http.StatusBadRequest)
				return errors.New(msg)
			} else if deploymentTarget.Deployment.ID != deploymentRequest.ID {
				msg := "given deployment does not match deployment of the given target"
				http.Error(w, msg, http.StatusBadRequest)
				return errors.New(msg)
			}

			if IsEmptyUUID(deploymentRequest.ID) {
				if err = db.CreateDeployment(ctx, &deploymentRequest); err != nil {
					log.Warn("could not create deployment", zap.Error(err))
					sentry.GetHubFromContext(ctx).CaptureException(err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return err
				}
			}

			if _, err := db.CreateDeploymentRevision(ctx, &deploymentRequest); err != nil {
				log.Warn("could not create deployment revision", zap.Error(err))
				sentry.GetHubFromContext(ctx).CaptureException(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}

			RespondJSON(w, make(map[string]any))
			// TODO might need to send a proper deployment object back, but not sure yet what it looks like
			return nil
		}
	})
}

func getDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deployment := internalctx.GetDeployment(ctx)
	if deploymentStatus, err := db.GetDeploymentStatus(ctx, deployment.ID, 100); err != nil {
		internalctx.GetLogger(ctx).Error("failed to get deploymentstatus", zap.Error(err))
		sentry.GetHubFromContext(ctx).CaptureException(err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		RespondJSON(w, deploymentStatus)
	}
}

func deploymentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		deploymentId, err := uuid.Parse(r.PathValue("deploymentId"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		deployment, err := db.GetDeployment(ctx, deploymentId)
		if errors.Is(err, apierrors.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else if err != nil {
			internalctx.GetLogger(r.Context()).Error("failed to get deployment", zap.Error(err))
			sentry.GetHubFromContext(ctx).CaptureException(err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			ctx = internalctx.WithDeployment(ctx, deployment)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}
