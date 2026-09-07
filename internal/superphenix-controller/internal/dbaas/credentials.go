package dbaas

import (
	"context"
	"fmt"
	"net/url"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetCredentials returns the connection information of a database: the
// credentials CloudNativePG generated for the application user, and the address
// clients should connect to. That address is the load balancer VIP when the
// database was given one, and the read-write service otherwise.
func GetCredentials(ctx context.Context, namespace, eid string) (view.DatabaseCredentialsView, error) {
	log := logger.GetLogger(ctx)

	// Resolving the database first enforces both the tenancy check and the
	// "managed by DBaaS" check before any secret is read.
	database, err := GetDatabase(ctx, namespace, eid)
	if err != nil {
		return view.DatabaseCredentialsView{}, err
	}

	secretName := eid + appSecretSuffix
	secret, err := k8s.K8sClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Err(err).Str("namespace", namespace).Str("eid", eid).Msg("Error getting database credentials")
		}
		return view.DatabaseCredentialsView{}, err
	}

	host, err := resolveHost(ctx, namespace, eid, database)
	if err != nil {
		return view.DatabaseCredentialsView{}, err
	}

	credentials := view.DatabaseCredentialsView{
		Host:     host,
		Port:     PostgresPort,
		Database: string(secret.Data[secretKeyDatabase]),
		Username: string(secret.Data[secretKeyUsername]),
		Password: string(secret.Data[secretKeyPassword]),
	}
	credentials.Uri = buildURI(credentials)

	return credentials, nil
}

// resolveHost prefers the load balancer VIP so the address stays valid across a
// failover and from outside the cluster; it falls back to the read-write service
// FQDN, which only resolves inside the overlay.
func resolveHost(ctx context.Context, namespace, eid string, database view.DatabaseView) (string, error) {
	log := logger.GetLogger(ctx)

	lb, err := k8s.KubeOvnClient.KubeovnV1().SwitchLBRules().Get(ctx, eid, metav1.GetOptions{})
	switch {
	case err == nil && lb.Spec.Vip != "":
		return lb.Spec.Vip, nil
	case err != nil && !apierrors.IsNotFound(err):
		// A broken lookup must not silently downgrade the answer to an address the
		// caller may not be able to reach.
		log.Err(err).Str("namespace", namespace).Str("eid", eid).Msg("Error getting database load balancer")
		return "", err
	}

	return fmt.Sprintf("%s.%s.svc", database.Status.WriteService, namespace), nil
}

func buildURI(c view.DatabaseCredentialsView) string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s",
		url.PathEscape(c.Username), url.PathEscape(c.Password), c.Host, c.Port, url.PathEscape(c.Database))
}
