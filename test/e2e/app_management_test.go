package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"path"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/v2/common"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/account"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	projectFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/project"
	repoFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/repos"
	"github.com/argoproj/argo-cd/v2/test/e2e/testdata"
	"github.com/argoproj/argo-cd/v2/util/argo"
	. "github.com/argoproj/argo-cd/v2/util/argo"
	. "github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	guestbookPath          = "guestbook"
	guestbookPathLocal     = "./testdata/guestbook_local"
	globalWithNoNameSpace  = "global-with-no-namespace"
	guestbookWithNamespace = "guestbook-with-namespace"
)

// This empty test is here only for clarity, to conform to logs rbac tests structure in account. This exact usecase is covered in the TestAppLogs test
func TestGetLogsAllowNoSwitch(t *testing.T) {
}

// There is some code duplication in the below GetLogs tests, the reason for that is to allow getting rid of most of those tests easily in the next release,
// when the temporary switch would die
func TestGetLogsDenySwitchOn(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")

	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "applications",
				Action:   "create",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "get",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "sync",
				Scope:    "*",
			},
			{
				Resource: "projects",
				Action:   "get",
				Scope:    "*",
			},
		}, "app-creator")

	GivenWithSameState(t).
		Path("guestbook-logs").
		When().
		CreateApp().
		Sync().
		SetParamInSettingConfigMap("server.rbac.log.enforce.enable", "true").
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			_, err := RunCli("app", "logs", app.Name, "--kind", "Deployment", "--group", "", "--name", "guestbook-ui")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "permission denied")
		})
}

func TestGetLogsAllowSwitchOn(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")

	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "applications",
				Action:   "create",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "get",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "sync",
				Scope:    "*",
			},
			{
				Resource: "projects",
				Action:   "get",
				Scope:    "*",
			},
			{
				Resource: "logs",
				Action:   "get",
				Scope:    "*",
			},
		}, "app-creator")

	GivenWithSameState(t).
		Path("guestbook-logs").
		When().
		CreateApp().
		Sync().
		SetParamInSettingConfigMap("server.rbac.log.enforce.enable", "true").
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Deployment", "--group", "", "--name", "guestbook-ui")
			assert.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Pod")
			assert.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Service")
			assert.NoError(t, err)
			assert.NotContains(t, out, "Hi")
		})

}

func TestGetLogsAllowSwitchOff(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")

	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "applications",
				Action:   "create",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "get",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "sync",
				Scope:    "*",
			},
			{
				Resource: "projects",
				Action:   "get",
				Scope:    "*",
			},
		}, "app-creator")

	Given(t).
		Path("guestbook-logs").
		When().
		CreateApp().
		Sync().
		SetParamInSettingConfigMap("server.rbac.log.enforce.enable", "false").
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Deployment", "--group", "", "--name", "guestbook-ui")
			assert.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Pod")
			assert.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Service")
			assert.NoError(t, err)
			assert.NotContains(t, out, "Hi")
		})
}

func TestSyncToUnsignedCommit(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestSyncToSignedCommitWithoutKnownKey(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestSyncToSignedCommitKeyWithKnownKey(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestAppCreation(t *testing.T) {
	ctx := Given(t)

	ctx.
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.Equal(t, Name(), app.Name)
			assert.Equal(t, RepoURL(RepoURLTypeFile), app.Spec.Source.RepoURL)
			assert.Equal(t, guestbookPath, app.Spec.Source.Path)
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			assert.NoError(t, err)
			assert.Contains(t, output, Name())
		}).
		When().
		// ensure that create is idempotent
		CreateApp().
		Then().
		Given().
		Revision("master").
		When().
		// ensure that update replaces spec and merge labels and annotations
		And(func() {
			FailOnErr(AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Patch(context.Background(),
				ctx.GetName(), types.MergePatchType, []byte(`{"metadata": {"labels": { "test": "label" }, "annotations": { "test": "annotation" }}}`), metav1.PatchOptions{}))
		}).
		CreateApp("--upsert").
		Then().
		And(func(app *Application) {
			assert.Equal(t, "label", app.Labels["test"])
			assert.Equal(t, "annotation", app.Annotations["test"])
			assert.Equal(t, "master", app.Spec.Source.TargetRevision)
		})
}

func TestAppCreationWithoutForceUpdate(t *testing.T) {
	ctx := Given(t)

	ctx.
		Path(guestbookPath).
		DestName("in-cluster").
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.Equal(t, Name(), app.Name)
			assert.Equal(t, RepoURL(RepoURLTypeFile), app.Spec.Source.RepoURL)
			assert.Equal(t, guestbookPath, app.Spec.Source.Path)
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, "in-cluster", app.Spec.Destination.Name)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			assert.NoError(t, err)
			assert.Contains(t, output, Name())
		}).
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "existing application spec is different, use upsert flag to force update"))
}

func TestDeleteAppResource(t *testing.T) {
	ctx := Given(t)

	ctx.
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			// app should be listed
			if _, err := RunCli("app", "delete-resource", Name(), "--kind", "Service", "--resource-name", "guestbook-ui"); err != nil {
				assert.NoError(t, err)
			}
		}).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing))
}

// demonstrate that we cannot use a standard sync when an immutable field is changed, we must use "force"
func TestImmutableChange(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	text := FailOnErr(Run(".", "kubectl", "get", "service", "-n", "kube-system", "kube-dns", "-o", "jsonpath={.spec.clusterIP}")).(string)
	parts := strings.Split(text, ".")
	n := rand.Intn(254)
	ip1 := fmt.Sprintf("%s.%s.%s.%d", parts[0], parts[1], parts[2], n)
	ip2 := fmt.Sprintf("%s.%s.%s.%d", parts[0], parts[1], parts[2], n+1)
	Given(t).
		Path("service").
		When().
		CreateApp().
		PatchFile("service.yaml", fmt.Sprintf(`[{"op": "add", "path": "/spec/clusterIP", "value": "%s"}]`, ip1)).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		PatchFile("service.yaml", fmt.Sprintf(`[{"op": "add", "path": "/spec/clusterIP", "value": "%s"}]`, ip2)).
		IgnoreErrors().
		Sync().
		DoNotIgnoreErrors().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceResultNumbering(1)).
		Expect(ResourceResultMatches(ResourceResult{
			Kind:      "Service",
			Version:   "v1",
			Namespace: DeploymentNamespace(),
			Name:      "my-service",
			SyncPhase: "Sync",
			Status:    "SyncFailed",
			HookPhase: "Failed",
			Message:   `Service "my-service" is invalid`,
		})).
		// now we can do this will a force
		Given().
		Force().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestInvalidAppProject(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		Project("does-not-exist").
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "application references project does-not-exist which does not exist"))
}

func TestAppDeletion(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist()).
		Expect(Event(EventReasonResourceDeleted, "delete"))

	output, err := RunCli("app", "list")
	assert.NoError(t, err)
	assert.NotContains(t, output, Name())
}

func TestAppLabels(t *testing.T) {
	Given(t).
		Path("config-map").
		When().
		CreateApp("-l", "foo=bar").
		Then().
		And(func(app *Application) {
			assert.Contains(t, FailOnErr(RunCli("app", "list")), Name())
			assert.Contains(t, FailOnErr(RunCli("app", "list", "-l", "foo=bar")), Name())
			assert.NotContains(t, FailOnErr(RunCli("app", "list", "-l", "foo=rubbish")), Name())
		}).
		Given().
		// remove both name and replace labels means nothing will sync
		Name("").
		When().
		IgnoreErrors().
		Sync("-l", "foo=rubbish").
		DoNotIgnoreErrors().
		Then().
		Expect(Error("", "no apps match selector foo=rubbish")).
		// check we can update the app and it is then sync'd
		Given().
		When().
		Sync("-l", "foo=bar")
}

func TestTrackAppStateAndSyncApp(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(Success(fmt.Sprintf("Service     %s  guestbook-ui  Synced ", DeploymentNamespace()))).
		Expect(Success(fmt.Sprintf("apps   Deployment  %s  guestbook-ui  Synced", DeploymentNamespace()))).
		Expect(Event(EventReasonResourceUpdated, "sync")).
		And(func(app *Application) {
			assert.NotNil(t, app.Status.OperationState.SyncResult)
		})
}

func TestAppRollbackSuccessful(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.NotEmpty(t, app.Status.Sync.Revision)
		}).
		And(func(app *Application) {
			appWithHistory := app.DeepCopy()
			appWithHistory.Status.History = []RevisionHistory{{
				ID:         1,
				Revision:   app.Status.Sync.Revision,
				DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-1 * time.Minute)},
				Source:     app.Spec.Source,
			}, {
				ID:         2,
				Revision:   "cdb",
				DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-2 * time.Minute)},
				Source:     app.Spec.Source,
			}}
			patch, _, err := diff.CreateTwoWayMergePatch(app, appWithHistory, &Application{})
			assert.NoError(t, err)

			app, err = AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Patch(context.Background(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			assert.NoError(t, err)

			// sync app and make sure it reaches InSync state
			_, err = RunCli("app", "rollback", app.Name, "1")
			assert.NoError(t, err)

		}).
		Expect(Event(EventReasonOperationStarted, "rollback")).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, SyncStatusCodeSynced, app.Status.Sync.Status)
			assert.NotNil(t, app.Status.OperationState.SyncResult)
			assert.Equal(t, 2, len(app.Status.OperationState.SyncResult.Resources))
			assert.Equal(t, OperationSucceeded, app.Status.OperationState.Phase)
			assert.Equal(t, 3, len(app.Status.History))
		})
}

func TestComparisonFailsIfClusterNotAdded(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		DestServer("https://not-registered-cluster/api").
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(DoesNotExist())
}

func TestCannotSetInvalidPath(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		IgnoreErrors().
		AppSet("--path", "garbage").
		Then().
		Expect(Error("", "app path does not exist"))
}

func TestManipulateApplicationResources(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			manifests, err := RunCli("app", "manifests", app.Name, "--source", "live")
			assert.NoError(t, err)
			resources, err := kube.SplitYAML([]byte(manifests))
			assert.NoError(t, err)

			index := -1
			for i := range resources {
				if resources[i].GetKind() == kube.DeploymentKind {
					index = i
					break
				}
			}
			assert.True(t, index > -1)

			deployment := resources[index]

			closer, client, err := ArgoCDClientset.NewApplicationClient()
			assert.NoError(t, err)
			defer io.Close(closer)

			_, err = client.DeleteResource(context.Background(), &applicationpkg.ApplicationResourceDeleteRequest{
				Name:         &app.Name,
				Group:        pointer.String(deployment.GroupVersionKind().Group),
				Kind:         pointer.String(deployment.GroupVersionKind().Kind),
				Version:      pointer.String(deployment.GroupVersionKind().Version),
				Namespace:    pointer.String(deployment.GetNamespace()),
				ResourceName: pointer.String(deployment.GetName()),
			})
			assert.NoError(t, err)
		}).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

func assetSecretDataHidden(t *testing.T, manifest string) {
	secret, err := UnmarshalToUnstructured(manifest)
	assert.NoError(t, err)

	_, hasStringData, err := unstructured.NestedMap(secret.Object, "stringData")
	assert.NoError(t, err)
	assert.False(t, hasStringData)

	secretData, hasData, err := unstructured.NestedMap(secret.Object, "data")
	assert.NoError(t, err)
	assert.True(t, hasData)
	for _, v := range secretData {
		assert.Regexp(t, regexp.MustCompile(`[*]*`), v)
	}
	var lastAppliedConfigAnnotation string
	annotations := secret.GetAnnotations()
	if annotations != nil {
		lastAppliedConfigAnnotation = annotations[v1.LastAppliedConfigAnnotation]
	}
	if lastAppliedConfigAnnotation != "" {
		assetSecretDataHidden(t, lastAppliedConfigAnnotation)
	}
}

func TestAppWithSecrets(t *testing.T) {
	closer, client, err := ArgoCDClientset.NewApplicationClient()
	assert.NoError(t, err)
	defer io.Close(closer)

	Given(t).
		Path("secrets").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res := FailOnErr(client.GetResource(context.Background(), &applicationpkg.ApplicationResourceRequest{
				Namespace:    &app.Spec.Destination.Namespace,
				Kind:         pointer.String(kube.SecretKind),
				Group:        pointer.String(""),
				Name:         &app.Name,
				Version:      pointer.String("v1"),
				ResourceName: pointer.String("test-secret"),
			})).(*applicationpkg.ApplicationResourceResponse)
			assetSecretDataHidden(t, res.GetManifest())

			manifests, err := client.GetManifests(context.Background(), &applicationpkg.ApplicationManifestQuery{Name: &app.Name})
			errors.CheckError(err)

			for _, manifest := range manifests.Manifests {
				assetSecretDataHidden(t, manifest)
			}

			diffOutput := FailOnErr(RunCli("app", "diff", app.Name)).(string)
			assert.Empty(t, diffOutput)

			// make sure resource update error does not print secret details
			_, err = RunCli("app", "patch-resource", "test-app-with-secrets", "--resource-name", "test-secret",
				"--kind", "Secret", "--patch", `{"op": "add", "path": "/data", "value": "hello"}'`,
				"--patch-type", "application/json-patch+json")
			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("failed to patch Secret %s/test-secret", DeploymentNamespace()))
			assert.NotContains(t, err.Error(), "username")
			assert.NotContains(t, err.Error(), "password")

			// patch secret and make sure app is out of sync and diff detects the change
			FailOnErr(KubeClientset.CoreV1().Secrets(DeploymentNamespace()).Patch(context.Background(),
				"test-secret", types.JSONPatchType, []byte(`[
	{"op": "remove", "path": "/data/username"},
	{"op": "add", "path": "/stringData", "value": {"password": "foo"}}
]`), metav1.PatchOptions{}))
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			diffOutput, err := RunCli("app", "diff", app.Name)
			assert.Error(t, err)
			assert.Contains(t, diffOutput, "username: ++++++++")
			assert.Contains(t, diffOutput, "password: ++++++++++++")

			// local diff should ignore secrets
			diffOutput = FailOnErr(RunCli("app", "diff", app.Name, "--local", "testdata/secrets")).(string)
			assert.Empty(t, diffOutput)

			// ignore missing field and make sure diff shows no difference
			app.Spec.IgnoreDifferences = []ResourceIgnoreDifferences{{
				Kind: kube.SecretKind, JSONPointers: []string{"/data"},
			}}
			FailOnErr(client.UpdateSpec(context.Background(), &applicationpkg.ApplicationUpdateSpecRequest{Name: &app.Name, Spec: &app.Spec}))
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			diffOutput := FailOnErr(RunCli("app", "diff", app.Name)).(string)
			assert.Empty(t, diffOutput)
		}).
		// verify not committed secret also ignore during diffing
		When().
		WriteFile("secret3.yaml", `
apiVersion: v1
kind: Secret
metadata:
  name: test-secret3
stringData:
  username: test-username`).
		Then().
		And(func(app *Application) {
			diffOutput := FailOnErr(RunCli("app", "diff", app.Name, "--local", "testdata/secrets")).(string)
			assert.Empty(t, diffOutput)
		})
}

func TestResourceDiffing(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Patch deployment
			_, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Patch(context.Background(),
				"guestbook-ui", types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "test" }]`), metav1.PatchOptions{})
			assert.NoError(t, err)
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			diffOutput, err := RunCli("app", "diff", app.Name, "--local", "testdata/guestbook")
			assert.Error(t, err)
			assert.Contains(t, diffOutput, fmt.Sprintf("===== apps/Deployment %s/guestbook-ui ======", DeploymentNamespace()))
		}).
		Given().
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {
			IgnoreDifferences: OverrideIgnoreDiff{JSONPointers: []string{"/spec/template/spec/containers/0/image"}},
		}}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			diffOutput, err := RunCli("app", "diff", app.Name, "--local", "testdata/guestbook")
			assert.NoError(t, err)
			assert.Empty(t, diffOutput)
		}).
		Given().
		When().
		And(func() {
			output, err := RunWithStdin(testdata.SSARevisionHistoryDeployment, "", "kubectl", "apply", "-n", DeploymentNamespace(), "--server-side=true", "--field-manager=revision-history-manager", "--validate=false", "--force-conflicts", "-f", "-")
			assert.NoError(t, err)
			assert.Contains(t, output, "serverside-applied")
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Given().
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {
			IgnoreDifferences: OverrideIgnoreDiff{
				ManagedFieldsManagers: []string{"revision-history-manager"},
				JSONPointers:          []string{"/spec/template/spec/containers/0/image"},
			},
		}}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Given().
		When().
		Sync().
		PatchApp(`[{
			"op": "add",
			"path": "/spec/syncPolicy",
			"value": { "syncOptions": ["RespectIgnoreDifferences=true"] }
			}]`).
		And(func() {
			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Equal(t, int32(3), *deployment.Spec.RevisionHistoryLimit)
		}).
		And(func() {
			output, err := RunWithStdin(testdata.SSARevisionHistoryDeployment, "", "kubectl", "apply", "-n", DeploymentNamespace(), "--server-side=true", "--field-manager=revision-history-manager", "--validate=false", "--force-conflicts", "-f", "-")
			assert.NoError(t, err)
			assert.Contains(t, output, "serverside-applied")
		}).
		Then().
		When().Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Equal(t, int32(1), *deployment.Spec.RevisionHistoryLimit)
		}).
		When().Sync().Then().Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Equal(t, int32(1), *deployment.Spec.RevisionHistoryLimit)
		})
}

func TestCRDs(t *testing.T) {
	testEdgeCasesApplicationResources(t, "crd-creation", health.HealthStatusHealthy)
}

func TestKnownTypesInCRDDiffing(t *testing.T) {
	dummiesGVR := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "dummies"}

	Given(t).
		Path("crd-creation").
		When().CreateApp().Sync().Then().
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		And(func() {
			dummyResIf := DynamicClientset.Resource(dummiesGVR).Namespace(DeploymentNamespace())
			patchData := []byte(`{"spec":{"cpu": "2"}}`)
			FailOnErr(dummyResIf.Patch(context.Background(), "dummy-crd-instance", types.MergePatchType, patchData, metav1.PatchOptions{}))
		}).Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		And(func() {
			SetResourceOverrides(map[string]ResourceOverride{
				"argoproj.io/Dummy": {
					KnownTypeFields: []KnownTypeField{{
						Field: "spec",
						Type:  "core/v1/ResourceList",
					}},
				},
			})
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestDuplicatedResources(t *testing.T) {
	testEdgeCasesApplicationResources(t, "duplicated-resources", health.HealthStatusHealthy)
}

func TestConfigMap(t *testing.T) {
	testEdgeCasesApplicationResources(t, "config-map", health.HealthStatusHealthy, "my-map  Synced                configmap/my-map created")
}

func testEdgeCasesApplicationResources(t *testing.T, appPath string, statusCode health.HealthStatusCode, message ...string) {
	expect := Given(t).
		Path(appPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
	for i := range message {
		expect = expect.Expect(Success(message[i]))
	}
	expect.
		Expect(HealthIs(statusCode)).
		And(func(app *Application) {
			diffOutput, err := RunCli("app", "diff", app.Name, "--local", path.Join("testdata", appPath))
			assert.Empty(t, diffOutput)
			assert.NoError(t, err)
		})
}

const actionsConfig = `discovery.lua: return { sample = {} }
definitions:
- name: sample
  action.lua: |
    obj.metadata.labels.sample = 'test'
    return obj`

func TestResourceAction(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {Actions: actionsConfig}}).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {

			closer, client, err := ArgoCDClientset.NewApplicationClient()
			assert.NoError(t, err)
			defer io.Close(closer)

			actions, err := client.ListResourceActions(context.Background(), &applicationpkg.ApplicationResourceRequest{
				Name:         &app.Name,
				Group:        pointer.String("apps"),
				Kind:         pointer.String("Deployment"),
				Version:      pointer.String("v1"),
				Namespace:    pointer.String(DeploymentNamespace()),
				ResourceName: pointer.String("guestbook-ui"),
			})
			assert.NoError(t, err)
			assert.Equal(t, []*ResourceAction{{Name: "sample", Disabled: false}}, actions.Actions)

			_, err = client.RunResourceAction(context.Background(), &applicationpkg.ResourceActionRunRequest{Name: &app.Name,
				Group:        pointer.String("apps"),
				Kind:         pointer.String("Deployment"),
				Version:      pointer.String("v1"),
				Namespace:    pointer.String(DeploymentNamespace()),
				ResourceName: pointer.String("guestbook-ui"),
				Action:       pointer.String("sample"),
			})
			assert.NoError(t, err)

			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			assert.NoError(t, err)

			assert.Equal(t, "test", deployment.Labels["sample"])
		})
}

func TestSyncResourceByLabel(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			_, _ = RunCli("app", "sync", app.Name, "--label", fmt.Sprintf("app.kubernetes.io/instance=%s", app.Name))
		}).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			_, err := RunCli("app", "sync", app.Name, "--label", "this-label=does-not-exist")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "level=fatal")
		})
}

func TestLocalManifestSync(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			res, _ := RunCli("app", "manifests", app.Name)
			assert.Contains(t, res, "containerPort: 80")
			assert.Contains(t, res, "image: quay.io/argoprojlabs/argocd-e2e-container:0.2")
		}).
		Given().
		LocalPath(guestbookPathLocal).
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res, _ := RunCli("app", "manifests", app.Name)
			assert.Contains(t, res, "containerPort: 81")
			assert.Contains(t, res, "image: quay.io/argoprojlabs/argocd-e2e-container:0.3")
		}).
		Given().
		LocalPath("").
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res, _ := RunCli("app", "manifests", app.Name)
			assert.Contains(t, res, "containerPort: 80")
			assert.Contains(t, res, "image: quay.io/argoprojlabs/argocd-e2e-container:0.2")
		})
}

func TestLocalSync(t *testing.T) {
	Given(t).
		// we've got to use Helm as this uses kubeVersion
		Path("helm").
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			FailOnErr(RunCli("app", "sync", app.Name, "--local", "testdata/helm"))
		})
}

func TestNoLocalSyncWithAutosyncEnabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "set", app.Name, "--sync-policy", "automated")
			assert.NoError(t, err)

			_, err = RunCli("app", "sync", app.Name, "--local", guestbookPathLocal)
			assert.Error(t, err)
		})
}

func TestLocalSyncDryRunWithAutosyncEnabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "set", app.Name, "--sync-policy", "automated")
			assert.NoError(t, err)

			appBefore := app.DeepCopy()
			_, err = RunCli("app", "sync", app.Name, "--dry-run", "--local", guestbookPathLocal)
			assert.NoError(t, err)

			appAfter := app.DeepCopy()
			assert.True(t, reflect.DeepEqual(appBefore, appAfter))
		})
}

func TestSyncAsync(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		Async(true).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

// assertResourceActions verifies if view/modify resource actions are successful/failing for given application
func assertResourceActions(t *testing.T, appName string, successful bool) {
	assertError := func(err error, message string) {
		if successful {
			assert.NoError(t, err)
		} else {
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), message)
			}
		}
	}

	closer, cdClient := ArgoCDClientset.NewApplicationClientOrDie()
	defer io.Close(closer)

	deploymentResource, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
	require.NoError(t, err)

	logs, err := cdClient.PodLogs(context.Background(), &applicationpkg.ApplicationPodLogsQuery{
		Group:        pointer.String("apps"),
		Kind:         pointer.String("Deployment"),
		Name:         &appName,
		Namespace:    pointer.String(DeploymentNamespace()),
		Container:    pointer.String(""),
		SinceSeconds: pointer.Int64(0),
		TailLines:    pointer.Int64(0),
		Follow:       pointer.Bool(false),
	})
	require.NoError(t, err)
	_, err = logs.Recv()
	assertError(err, "EOF")

	expectedError := fmt.Sprintf("Deployment apps guestbook-ui not found as part of application %s", appName)

	_, err = cdClient.ListResourceEvents(context.Background(), &applicationpkg.ApplicationResourceEventsQuery{
		Name:              &appName,
		ResourceName:      pointer.String("guestbook-ui"),
		ResourceNamespace: pointer.String(DeploymentNamespace()),
		ResourceUID:       pointer.String(string(deploymentResource.UID)),
	})
	assertError(err, fmt.Sprintf("%s not found as part of application %s", "guestbook-ui", appName))

	_, err = cdClient.GetResource(context.Background(), &applicationpkg.ApplicationResourceRequest{
		Name:         &appName,
		ResourceName: pointer.String("guestbook-ui"),
		Namespace:    pointer.String(DeploymentNamespace()),
		Version:      pointer.String("v1"),
		Group:        pointer.String("apps"),
		Kind:         pointer.String("Deployment"),
	})
	assertError(err, expectedError)

	_, err = cdClient.RunResourceAction(context.Background(), &applicationpkg.ResourceActionRunRequest{
		Name:         &appName,
		ResourceName: pointer.String("guestbook-ui"),
		Namespace:    pointer.String(DeploymentNamespace()),
		Version:      pointer.String("v1"),
		Group:        pointer.String("apps"),
		Kind:         pointer.String("Deployment"),
		Action:       pointer.String("restart"),
	})
	assertError(err, expectedError)

	_, err = cdClient.DeleteResource(context.Background(), &applicationpkg.ApplicationResourceDeleteRequest{
		Name:         &appName,
		ResourceName: pointer.String("guestbook-ui"),
		Namespace:    pointer.String(DeploymentNamespace()),
		Version:      pointer.String("v1"),
		Group:        pointer.String("apps"),
		Kind:         pointer.String("Deployment"),
	})
	assertError(err, expectedError)
}

func TestPermissions(t *testing.T) {
	appCtx := Given(t)
	projName := "argo-project"
	projActions := projectFixture.
		Given(t).
		Name(projName).
		When().
		Create()

	sourceError := fmt.Sprintf("application repo %s is not permitted in project 'argo-project'", RepoURL(RepoURLTypeFile))
	destinationError := fmt.Sprintf("application destination {%s %s} is not permitted in project 'argo-project'", KubernetesInternalAPIServerAddr, DeploymentNamespace())

	appCtx.
		Path("guestbook-logs").
		Project(projName).
		When().
		IgnoreErrors().
		// ensure app is not created if project permissions are missing
		CreateApp().
		Then().
		Expect(Error("", sourceError)).
		Expect(Error("", destinationError)).
		When().
		DoNotIgnoreErrors().
		// add missing permissions, create and sync app
		And(func() {
			projActions.AddDestination("*", "*")
			projActions.AddSource("*")
		}).
		CreateApp().
		Sync().
		Then().
		// make sure application resource actiions are successful
		And(func(app *Application) {
			assertResourceActions(t, app.Name, true)
		}).
		When().
		// remove projet permissions and "refresh" app
		And(func() {
			projActions.UpdateProject(func(proj *AppProject) {
				proj.Spec.Destinations = nil
				proj.Spec.SourceRepos = nil
			})
		}).
		Refresh(RefreshTypeNormal).
		Then().
		// ensure app resource tree is empty when source/destination permissions are missing
		Expect(Condition(ApplicationConditionInvalidSpecError, destinationError)).
		Expect(Condition(ApplicationConditionInvalidSpecError, sourceError)).
		And(func(app *Application) {
			closer, cdClient := ArgoCDClientset.NewApplicationClientOrDie()
			defer io.Close(closer)
			tree, err := cdClient.ResourceTree(context.Background(), &applicationpkg.ResourcesQuery{ApplicationName: &app.Name})
			require.NoError(t, err)
			assert.Len(t, tree.Nodes, 0)
			assert.Len(t, tree.OrphanedNodes, 0)
		}).
		When().
		// add missing permissions but deny management of Deployment kind
		And(func() {
			projActions.
				AddDestination("*", "*").
				AddSource("*").
				UpdateProject(func(proj *AppProject) {
					proj.Spec.NamespaceResourceBlacklist = []metav1.GroupKind{{Group: "*", Kind: "Deployment"}}
				})
		}).
		Refresh(RefreshTypeNormal).
		Then().
		// make sure application resource actiions are failing
		And(func(app *Application) {
			assertResourceActions(t, "test-permissions", false)
		})
}

func TestPermissionWithScopedRepo(t *testing.T) {
	projName := "argo-project"
	projectFixture.
		Given(t).
		Name(projName).
		Destination("*,*").
		When().
		Create().
		AddSource("*")

	repoFixture.Given(t, true).
		When().
		Path(RepoURL(RepoURLTypeFile)).
		Project(projName).
		Create()

	GivenWithSameState(t).
		Project(projName).
		RepoURLType(RepoURLTypeFile).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Prune=false"}}]`).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		DeleteFile("pod-1.yaml").
		Refresh(RefreshTypeHard).
		IgnoreErrors().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceSyncStatusIs("Pod", "pod-1", SyncStatusCodeOutOfSync))
}

func TestPermissionDeniedWithScopedRepo(t *testing.T) {
	projName := "argo-project"
	projectFixture.
		Given(t).
		Name(projName).
		Destination("*,*").
		When().
		Create()

	repoFixture.Given(t, true).
		When().
		Path(RepoURL(RepoURLTypeFile)).
		Create()

	GivenWithSameState(t).
		Project(projName).
		RepoURLType(RepoURLTypeFile).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Prune=false"}}]`).
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "is not permitted in project"))
}

func TestPermissionDeniedWithNegatedNamespace(t *testing.T) {
	projName := "argo-project"
	projectFixture.
		Given(t).
		Name(projName).
		Destination("*,!*test-permission-denied-with-negated-namespace*").
		When().
		Create()

	repoFixture.Given(t, true).
		When().
		Path(RepoURL(RepoURLTypeFile)).
		Project(projName).
		Create()

	GivenWithSameState(t).
		Project(projName).
		RepoURLType(RepoURLTypeFile).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Prune=false"}}]`).
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "is not permitted in project"))
}

func TestPermissionDeniedWithNegatedServer(t *testing.T) {
	projName := "argo-project"
	projectFixture.
		Given(t).
		Name(projName).
		Destination("!https://kubernetes.default.svc,*").
		When().
		Create()

	repoFixture.Given(t, true).
		When().
		Path(RepoURL(RepoURLTypeFile)).
		Project(projName).
		Create()

	GivenWithSameState(t).
		Project(projName).
		RepoURLType(RepoURLTypeFile).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Prune=false"}}]`).
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "is not permitted in project"))
}

// make sure that if we deleted a resource from the app, it is not pruned if annotated with Prune=false
func TestSyncOptionPruneFalse(t *testing.T) {
	Given(t).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Prune=false"}}]`).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		DeleteFile("pod-1.yaml").
		Refresh(RefreshTypeHard).
		IgnoreErrors().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceSyncStatusIs("Pod", "pod-1", SyncStatusCodeOutOfSync))
}

// make sure that if we have an invalid manifest, we can add it if we disable validation, we get a server error rather than a client error
func TestSyncOptionValidateFalse(t *testing.T) {

	Given(t).
		Path("crd-validation").
		When().
		CreateApp().
		Then().
		Expect(Success("")).
		When().
		IgnoreErrors().
		Sync().
		Then().
		// client error
		Expect(Error("error validating data", "")).
		When().
		PatchFile("deployment.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Validate=false"}}]`).
		Sync().
		Then().
		// server error
		Expect(Error("cannot be handled as a Deployment", ""))
}

// make sure that, if we have a resource that needs pruning, but we're ignoring it, the app is in-sync
func TestCompareOptionIgnoreExtraneous(t *testing.T) {
	Given(t).
		Prune(false).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/compare-options": "IgnoreExtraneous"}}]`).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		DeleteFile("pod-1.yaml").
		Refresh(RefreshTypeHard).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Len(t, app.Status.Resources, 2)
			statusByName := map[string]SyncStatusCode{}
			for _, r := range app.Status.Resources {
				statusByName[r.Name] = r.Status
			}
			assert.Equal(t, SyncStatusCodeOutOfSync, statusByName["pod-1"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["pod-2"])
		}).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestSelfManagedApps(t *testing.T) {

	Given(t).
		Path("self-managed-app").
		When().
		PatchFile("resources.yaml", fmt.Sprintf(`[{"op": "replace", "path": "/spec/source/repoURL", "value": "%s"}]`, RepoURL(RepoURLTypeFile))).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(a *Application) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()

			reconciledCount := 0
			var lastReconciledAt *metav1.Time
			for event := range ArgoCDClientset.WatchApplicationWithRetry(ctx, a.Name, a.ResourceVersion) {
				reconciledAt := event.Application.Status.ReconciledAt
				if reconciledAt == nil {
					reconciledAt = &metav1.Time{}
				}
				if lastReconciledAt != nil && !lastReconciledAt.Equal(reconciledAt) {
					reconciledCount = reconciledCount + 1
				}
				lastReconciledAt = reconciledAt
			}

			assert.True(t, reconciledCount < 3, "Application was reconciled too many times")
		})
}

func TestExcludedResource(t *testing.T) {
	Given(t).
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {Actions: actionsConfig}}).
		Path(guestbookPath).
		ResourceFilter(settings.ResourcesFilter{
			ResourceExclusions: []settings.FilteredResource{{Kinds: []string{kube.DeploymentKind}}},
		}).
		When().
		CreateApp().
		Sync().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionExcludedResourceWarning, "Resource apps/Deployment guestbook-ui is excluded in the settings"))
}

func TestRevisionHistoryLimit(t *testing.T) {
	Given(t).
		Path("config-map").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Len(t, app.Status.History, 1)
		}).
		When().
		AppSet("--revision-history-limit", "1").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Len(t, app.Status.History, 1)
		})
}

func TestOrphanedResource(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	Given(t).
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true)},
		}).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		When().
		And(func() {
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "orphaned-configmap",
				},
			}, metav1.CreateOptions{}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionOrphanedResourceWarning, "Application has 1 orphaned resources")).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true), Ignore: []OrphanedResourceKey{{Group: "Test", Kind: "ConfigMap"}}},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionOrphanedResourceWarning, "Application has 1 orphaned resources")).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true), Ignore: []OrphanedResourceKey{{Kind: "ConfigMap"}}},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.NotContains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true), Ignore: []OrphanedResourceKey{{Kind: "ConfigMap", Name: "orphaned-configmap"}}},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.NotContains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: nil,
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions())
}

func TestNotPermittedResources(t *testing.T) {
	ctx := Given(t)

	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sample-ingress",
			Labels: map[string]string{
				common.LabelKeyAppInstance: ctx.GetName(),
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path: "/",
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "guestbook-ui",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
							PathType: &pathType,
						}},
					},
				},
			}},
		},
	}
	defer func() {
		log.Infof("Ingress 'sample-ingress' deleted from %s", ArgoCDNamespace)
		CheckError(KubeClientset.NetworkingV1().Ingresses(ArgoCDNamespace).Delete(context.Background(), "sample-ingress", metav1.DeleteOptions{}))
	}()

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "guestbook-ui",
			Labels: map[string]string{
				common.LabelKeyAppInstance: ctx.GetName(),
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Port:       80,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
			}},
			Selector: map[string]string{
				"app": "guestbook-ui",
			},
		},
	}

	ctx.ProjectSpec(AppProjectSpec{
		SourceRepos:  []string{"*"},
		Destinations: []ApplicationDestination{{Namespace: DeploymentNamespace(), Server: "*"}},
		NamespaceResourceBlacklist: []metav1.GroupKind{
			{Group: "", Kind: "Service"},
		}}).
		And(func() {
			FailOnErr(KubeClientset.NetworkingV1().Ingresses(ArgoCDNamespace).Create(context.Background(), ingress, metav1.CreateOptions{}))
			FailOnErr(KubeClientset.CoreV1().Services(DeploymentNamespace()).Create(context.Background(), svc, metav1.CreateOptions{}))
		}).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			statusByKind := make(map[string]ResourceStatus)
			for _, res := range app.Status.Resources {
				statusByKind[res.Kind] = res
			}
			_, hasIngress := statusByKind[kube.IngressKind]
			assert.False(t, hasIngress, "Ingress is prohibited not managed object and should be even visible to user")
			serviceStatus := statusByKind[kube.ServiceKind]
			assert.Equal(t, serviceStatus.Status, SyncStatusCodeUnknown, "Service is prohibited managed resource so should be set to Unknown")
			deploymentStatus := statusByKind[kube.DeploymentKind]
			assert.Equal(t, deploymentStatus.Status, SyncStatusCodeOutOfSync)
		}).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist())

	// Make sure prohibited resources are not deleted during application deletion
	FailOnErr(KubeClientset.NetworkingV1().Ingresses(ArgoCDNamespace).Get(context.Background(), "sample-ingress", metav1.GetOptions{}))
	FailOnErr(KubeClientset.CoreV1().Services(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{}))
}

func TestSyncWithInfos(t *testing.T) {
	expectedInfo := make([]*Info, 2)
	expectedInfo[0] = &Info{Name: "name1", Value: "val1"}
	expectedInfo[1] = &Info{Name: "name2", Value: "val2"}

	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "sync", app.Name,
				"--info", fmt.Sprintf("%s=%s", expectedInfo[0].Name, expectedInfo[0].Value),
				"--info", fmt.Sprintf("%s=%s", expectedInfo[1].Name, expectedInfo[1].Value))
			assert.NoError(t, err)
		}).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.ElementsMatch(t, app.Status.OperationState.Operation.Info, expectedInfo)
		})
}

//Given: argocd app create does not provide --dest-namespace
//       Manifest contains resource console which does not require namespace
//Expect: no app.Status.Conditions
func TestCreateAppWithNoNameSpaceForGlobalResource(t *testing.T) {
	Given(t).
		Path(globalWithNoNameSpace).
		When().
		CreateWithNoNameSpace().
		Then().
		And(func(app *Application) {
			time.Sleep(500 * time.Millisecond)
			app, err := AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Get(context.Background(), app.Name, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Len(t, app.Status.Conditions, 0)
		})
}

//Given: argocd app create does not provide --dest-namespace
//       Manifest contains resource deployment, and service which requires namespace
//       Deployment and service do not have namespace in manifest
//Expect: app.Status.Conditions for deployment ans service which does not have namespace in manifest
func TestCreateAppWithNoNameSpaceWhenRequired(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateWithNoNameSpace().
		Refresh(RefreshTypeNormal).
		Then().
		And(func(app *Application) {
			updatedApp, err := AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Get(context.Background(), app.Name, metav1.GetOptions{})
			require.NoError(t, err)

			assert.Len(t, updatedApp.Status.Conditions, 2)
			assert.Equal(t, updatedApp.Status.Conditions[0].Type, ApplicationConditionInvalidSpecError)
			assert.Equal(t, updatedApp.Status.Conditions[1].Type, ApplicationConditionInvalidSpecError)
		})
}

//Given: argocd app create does not provide --dest-namespace
//       Manifest contains resource deployment, and service which requires namespace
//       Some deployment and service has namespace in manifest
//       Some deployment and service does not have namespace in manifest
//Expect: app.Status.Conditions for deployment and service which does not have namespace in manifest
func TestCreateAppWithNoNameSpaceWhenRequired2(t *testing.T) {
	Given(t).
		Path(guestbookWithNamespace).
		When().
		CreateWithNoNameSpace().
		Refresh(RefreshTypeNormal).
		Then().
		And(func(app *Application) {
			updatedApp, err := AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Get(context.Background(), app.Name, metav1.GetOptions{})
			require.NoError(t, err)

			assert.Len(t, updatedApp.Status.Conditions, 2)
			assert.Equal(t, updatedApp.Status.Conditions[0].Type, ApplicationConditionInvalidSpecError)
			assert.Equal(t, updatedApp.Status.Conditions[1].Type, ApplicationConditionInvalidSpecError)
		})
}

func TestListResource(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	Given(t).
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true)},
		}).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		When().
		And(func() {
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "orphaned-configmap",
				},
			}, metav1.CreateOptions{}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionOrphanedResourceWarning, "Application has 1 orphaned resources")).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
			assert.Contains(t, output, "guestbook-ui")
		}).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name, "--orphaned=true")
			assert.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
			assert.NotContains(t, output, "guestbook-ui")
		}).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name, "--orphaned=false")
			assert.NoError(t, err)
			assert.NotContains(t, output, "orphaned-configmap")
			assert.Contains(t, output, "guestbook-ui")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: nil,
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions())
}

// Given application is set with --sync-option CreateNamespace=true
//       application --dest-namespace does not exist
// Verity application --dest-namespace is created
//        application sync successful
//        when application is deleted, --dest-namespace is not deleted
func TestNamespaceAutoCreation(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	updatedNamespace := getNewNamespace(t)
	defer func() {
		if !t.Skipped() {
			_, err := Run("", "kubectl", "delete", "namespace", updatedNamespace)
			assert.NoError(t, err)
		}
	}()
	Given(t).
		Timeout(30).
		Path("guestbook").
		When().
		CreateApp("--sync-option", "CreateNamespace=true").
		Then().
		And(func(app *Application) {
			//Make sure the namespace we are about to update to does not exist
			_, err := Run("", "kubectl", "get", "namespace", updatedNamespace)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
		}).
		When().
		AppSet("--dest-namespace", updatedNamespace).
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, health.HealthStatusHealthy)).
		Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, SyncStatusCodeSynced)).
		When().
		Delete(true).
		Then().
		Expect(Success("")).
		And(func(app *Application) {
			//Verify delete app does not delete the namespace auto created
			output, err := Run("", "kubectl", "get", "namespace", updatedNamespace)
			assert.NoError(t, err)
			assert.Contains(t, output, updatedNamespace)
		})
}

func TestFailedSyncWithRetry(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PreSync"}}]`).
		// make hook fail
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command", "value": ["false"]}]`).
		CreateApp().
		IgnoreErrors().
		Sync("--retry-limit=1", "--retry-backoff-duration=1s").
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(OperationMessageContains("retried 1 times"))
}

func TestCreateDisableValidation(t *testing.T) {
	Given(t).
		Path("baddir").
		When().
		CreateApp("--validate=false").
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "create", app.Name, "--upsert", "--validate=false", "--repo", RepoURL(RepoURLTypeFile),
				"--path", "baddir2", "--project", app.Spec.Project, "--dest-server", KubernetesInternalAPIServerAddr, "--dest-namespace", DeploymentNamespace())
			assert.NoError(t, err)
		}).
		When().
		AppSet("--path", "baddir3", "--validate=false")

}

func TestCreateFromPartialFile(t *testing.T) {
	partialApp :=
		`metadata:
  labels:
    labels.local/from-file: file
    labels.local/from-args: file
  annotations:
    annotations.local/from-file: file
  finalizers:
  - resources-finalizer.argocd.argoproj.io
spec:
  syncPolicy:
    automated:
      prune: true
`

	path := "helm-values"
	Given(t).
		When().
		// app should be auto-synced once created
		CreateFromPartialFile(partialApp, "--path", path, "-l", "labels.local/from-args=args", "--helm-set", "foo=foo").
		Then().
		Expect(Success("")).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			assert.Equal(t, map[string]string{"labels.local/from-file": "file", "labels.local/from-args": "args"}, app.ObjectMeta.Labels)
			assert.Equal(t, map[string]string{"annotations.local/from-file": "file"}, app.ObjectMeta.Annotations)
			assert.Equal(t, []string{"resources-finalizer.argocd.argoproj.io"}, app.ObjectMeta.Finalizers)
			assert.Equal(t, path, app.Spec.Source.Path)
			assert.Equal(t, []HelmParameter{{Name: "foo", Value: "foo"}}, app.Spec.Source.Helm.Parameters)
		})
}

// Ensure actions work when using a resource action that modifies status and/or spec
func TestCRDStatusSubresourceAction(t *testing.T) {
	actions := `
discovery.lua: |
  actions = {}
  actions["update-spec"] = {["disabled"] = false}
  actions["update-status"] = {["disabled"] = false}
  actions["update-both"] = {["disabled"] = false}
  return actions
definitions:
- name: update-both
  action.lua: |
    obj.spec = {}
    obj.spec.foo = "update-both"
    obj.status = {}
    obj.status.bar = "update-both"
    return obj
- name: update-spec
  action.lua: |
    obj.spec = {}
    obj.spec.foo = "update-spec"
    return obj
- name: update-status
  action.lua: |
    obj.status = {}
    obj.status.bar = "update-status"
    return obj
`
	Given(t).
		Path("crd-subresource").
		And(func() {
			SetResourceOverrides(map[string]ResourceOverride{
				"argoproj.io/StatusSubResource": {
					Actions: actions,
				},
				"argoproj.io/NonStatusSubResource": {
					Actions: actions,
				},
			})
		}).
		When().CreateApp().Sync().Then().
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		// tests resource actions on a CRD using status subresource
		And(func(app *Application) {
			_, err := RunCli("app", "actions", "run", app.Name, "--kind", "StatusSubResource", "update-both")
			assert.NoError(t, err)
			text := FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-both", text)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-both", text)

			_, err = RunCli("app", "actions", "run", app.Name, "--kind", "StatusSubResource", "update-spec")
			assert.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-spec", text)

			_, err = RunCli("app", "actions", "run", app.Name, "--kind", "StatusSubResource", "update-status")
			assert.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-status", text)
		}).
		// tests resource actions on a CRD *not* using status subresource
		And(func(app *Application) {
			_, err := RunCli("app", "actions", "run", app.Name, "--kind", "NonStatusSubResource", "update-both")
			assert.NoError(t, err)
			text := FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-both", text)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-both", text)

			_, err = RunCli("app", "actions", "run", app.Name, "--kind", "NonStatusSubResource", "update-spec")
			assert.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-spec", text)

			_, err = RunCli("app", "actions", "run", app.Name, "--kind", "NonStatusSubResource", "update-status")
			assert.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-status", text)
		})
}

func TestAppLogs(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	Given(t).
		Path("guestbook-logs").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Deployment", "--group", "", "--name", "guestbook-ui")
			assert.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Pod")
			assert.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Service")
			assert.NoError(t, err)
			assert.NotContains(t, out, "Hi")
		})
}

func TestAppWaitOperationInProgress(t *testing.T) {
	Given(t).
		And(func() {
			SetResourceOverrides(map[string]ResourceOverride{
				"batch/Job": {
					HealthLua: `return { status = 'Running' }`,
				},
				"apps/Deployment": {
					HealthLua: `return { status = 'Suspended' }`,
				},
			})
		}).
		Async(true).
		Path("hook-and-deployment").
		When().
		CreateApp().
		Sync().
		Then().
		// stuck in running state
		Expect(OperationPhaseIs(OperationRunning)).
		When().
		And(func() {
			_, err := RunCli("app", "wait", Name(), "--suspended")
			errors.CheckError(err)
		})
}

func TestSyncOptionReplace(t *testing.T) {
	Given(t).
		Path("config-map").
		When().
		PatchFile("config-map.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Replace=true"}}]`).
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, app.Status.OperationState.SyncResult.Resources[0].Message, "configmap/my-map created")
		}).
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, app.Status.OperationState.SyncResult.Resources[0].Message, "configmap/my-map replaced")
		})
}

func TestSyncOptionReplaceFromCLI(t *testing.T) {
	Given(t).
		Path("config-map").
		Replace().
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, app.Status.OperationState.SyncResult.Resources[0].Message, "configmap/my-map created")
		}).
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, app.Status.OperationState.SyncResult.Resources[0].Message, "configmap/my-map replaced")
		})
}

func TestDiscoverNewCommit(t *testing.T) {
	var sha string
	Given(t).
		Path("config-map").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			sha = app.Status.Sync.Revision
			assert.NotEmpty(t, sha)
		}).
		When().
		PatchFile("config-map.yaml", `[{"op": "replace", "path": "/data/foo", "value": "hello"}]`).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// make sure new commit is not discovered immediately after push
		And(func(app *Application) {
			assert.Equal(t, sha, app.Status.Sync.Revision)
		}).
		When().
		// make sure new commit is not discovered after refresh is requested
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.NotEqual(t, sha, app.Status.Sync.Revision)
		})
}

func TestDisableManifestGeneration(t *testing.T) {
	Given(t).
		Path("guestbook").
		When().
		CreateApp().
		Refresh(RefreshTypeHard).
		Then().
		And(func(app *Application) {
			assert.Equal(t, app.Status.SourceType, ApplicationSourceTypeKustomize)
		}).
		When().
		And(func() {
			SetEnableManifestGeneration(map[ApplicationSourceType]bool{
				ApplicationSourceTypeKustomize: false,
			})
		}).
		Refresh(RefreshTypeHard).
		Then().
		And(func(app *Application) {
			time.Sleep(1 * time.Second)
		}).
		And(func(app *Application) {
			assert.Equal(t, app.Status.SourceType, ApplicationSourceTypeDirectory)
		})
}

func TestSwitchTrackingMethod(t *testing.T) {
	ctx := Given(t)

	ctx.
		SetTrackingMethod(string(argo.TrackingMethodAnnotation)).
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Add resource with tracking annotation. This should put the
			// application OutOfSync.
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-configmap",
					Annotations: map[string]string{
						common.AnnotationKeyAppInstance: fmt.Sprintf("%s:/ConfigMap:%s/other-configmap", Name(), DeploymentNamespace()),
					},
				},
			}, metav1.CreateOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Delete resource to bring application back in sync
			FailOnErr(nil, KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Delete(context.Background(), "other-configmap", metav1.DeleteOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		SetTrackingMethod(string(argo.TrackingMethodLabel)).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Add a resource with a tracking annotation. This should not
			// affect the application, because we now use the tracking method
			// "label".
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-configmap",
					Annotations: map[string]string{
						common.AnnotationKeyAppInstance: fmt.Sprintf("%s:/ConfigMap:%s/other-configmap", Name(), DeploymentNamespace()),
					},
				},
			}, metav1.CreateOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Add a resource with the tracking label. The app should become
			// OutOfSync.
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "extra-configmap",
					Labels: map[string]string{
						common.LabelKeyAppInstance: Name(),
					},
				},
			}, metav1.CreateOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Delete resource to bring application back in sync
			FailOnErr(nil, KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Delete(context.Background(), "extra-configmap", metav1.DeleteOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestSwitchTrackingLabel(t *testing.T) {
	ctx := Given(t)

	ctx.
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Add extra resource that carries the default tracking label
			// We expect the app to go out of sync.
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-configmap",
					Labels: map[string]string{
						common.LabelKeyAppInstance: Name(),
					},
				},
			}, metav1.CreateOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Delete resource to bring application back in sync
			FailOnErr(nil, KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Delete(context.Background(), "other-configmap", metav1.DeleteOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		// Change tracking label
		SetTrackingLabel("argocd.tracking").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Create resource with the new tracking label, the application
			// is expected to go out of sync
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-configmap",
					Labels: map[string]string{
						"argocd.tracking": Name(),
					},
				},
			}, metav1.CreateOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Delete resource to bring application back in sync
			FailOnErr(nil, KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Delete(context.Background(), "other-configmap", metav1.DeleteOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Add extra resource that carries the default tracking label
			// We expect the app to stay in sync, because the configured
			// label is different.
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-configmap",
					Labels: map[string]string{
						common.LabelKeyAppInstance: Name(),
					},
				},
			}, metav1.CreateOptions{}))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestAnnotationTrackingExtraResources(t *testing.T) {
	ctx := Given(t)

	SetTrackingMethod(string(argo.TrackingMethodAnnotation))
	ctx.
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Add a resource with an annotation that is not referencing the
			// resource.
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "extra-configmap",
					Annotations: map[string]string{
						common.AnnotationKeyAppInstance: fmt.Sprintf("%s:apps/Deployment:%s/guestbook-cm", Name(), DeploymentNamespace()),
					},
				},
			}, metav1.CreateOptions{}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		And(func() {
			// Add a resource with an annotation that is self-referencing the
			// resource.
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-configmap",
					Annotations: map[string]string{
						common.AnnotationKeyAppInstance: fmt.Sprintf("%s:/ConfigMap:%s/other-configmap", Name(), DeploymentNamespace()),
					},
				},
			}, metav1.CreateOptions{}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusHealthy))
}
