package cleanup

import (
	"context"
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) ListResource(ctx context.Context, _, kind, namespace string, _ interface{}) ([]unstructured.Unstructured, error) {
	args := m.Called(ctx, kind, namespace)
	return args.Get(0).([]unstructured.Unstructured), args.Error(1)
}

func (m *MockClient) DeleteResource(ctx context.Context, groupVersion, kind, namespace, name string, cascade bool, opts metav1.DeleteOptions) error {
	args := m.Called(ctx, groupVersion, kind, namespace, name)
	return args.Error(0)
}

func (m *MockClient) ApplyResource(ctx context.Context, groupVersion, kind, namespace, name string, obj interface{}, cascade bool, opts string, extraOpts ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, groupVersion, kind, namespace, name, obj, cascade, opts, extraOpts)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

type MockEventGen struct {
	mock.Mock
}

func (m *MockEventGen) Add(events event.Info) {
	m.Called(events)
}

func TestCleanup(t *testing.T) {
	tests := []struct {
		name           string
		mockResources  []unstructured.Unstructured
		listErr        error
		deleteErr      error
		applyErr       error
		expectedError  bool
		expectedEvents int
	}{
		{
			name: "Successful Cleanup",
			mockResources: []unstructured.Unstructured{
				{Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "test-namespace",
					},
				}},
			},
			expectedError: false,
		},
		{
			name:          "Empty Resource List",
			mockResources: nil,
			expectedError: false,
		},
		{
			name:          "ListResource Fails",
			listErr:       errors.New("list error"),
			expectedError: true,
		},
		{
			name: "DeleteResource Fails",
			mockResources: []unstructured.Unstructured{
				{Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "test-namespace",
					},
				}},
			},
			deleteErr:     errors.New("deletion error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockClient := new(MockClient)
			mockEventGen := new(MockEventGen)
			mockLogger := logging.GlobalLogger()

			policy := &kyvernov2.CleanupPolicy{
				Spec: kyvernov2.CleanupPolicySpec{
					MatchResources: kyvernov2.MatchResources{
						All: kyvernov1.ResourceFilters{
							{
								ResourceDescription: kyvernov1.ResourceDescription{
									Kinds: []string{"Pod"},
								},
							},
						},
					},
				},
			}

			mockClient.On("ListResource", mock.Anything, "Pod", "").Return(tt.mockResources, tt.listErr)
			if tt.mockResources != nil && len(tt.mockResources) > 0 {
				mockClient.On("DeleteResource", mock.Anything, "v1", "Pod", "test-namespace", "test-pod").Return(tt.deleteErr)
			}

			ctrl := &controller{
				client:   mockClient,
				eventGen: mockEventGen,
			}

			err := ctrl.cleanup(context.Background(), mockLogger, policy)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
			mockEventGen.AssertExpectations(t)
		})
	}
}
