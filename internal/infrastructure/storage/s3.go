package storage

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// dropAcceptEncoding removes the Accept-Encoding header before signing. The
// SDK signs "Accept-Encoding: identity", and Google Cloud Storage's front end
// rewrites that header to "identity,gzip(gfe)" before verifying, so every
// signed request fails with SignatureDoesNotMatch. Unsigned, the transport
// adds its own value and the store ignores it.
var dropAcceptEncoding = middleware.FinalizeMiddlewareFunc("DropAcceptEncoding",
	func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
		if req, ok := in.Request.(*smithyhttp.Request); ok {
			req.Header.Del("Accept-Encoding")
		}
		return next.HandleFinalize(ctx, in)
	})

type Client struct {
	Bucket string
	// PublicBaseURL, when set, overrides the default public URL used by
	// PutPublic (needed for MinIO / R2 / self-hosted S3 that don't serve at
	// s3.amazonaws.com). Empty falls back to the AWS virtual-hosted URL.
	PublicBaseURL string
	*s3.Client
}

func NewClient(ctx context.Context, cfg aws.Config, bucket string) (*Client, error) {
	return &Client{
		Bucket: bucket,
		Client: s3.NewFromConfig(cfg, func(o *s3.Options) {
			// Custom endpoints (LocalStack, MinIO) need path-style requests:
			// virtual-hosted addressing puts the bucket in the hostname
			// (bucket.localhost:4566), which these servers don't resolve as a
			// bucket. Real AWS (no endpoint override) keeps the default.
			if os.Getenv("AWS_ENDPOINT_URL") != "" || os.Getenv("AWS_ENDPOINT_URL_S3") != "" {
				o.UsePathStyle = true
				// S3-compatible stores (Cloud Storage, MinIO, R2) reject the
				// SDK's default CRC checksums and chunked streaming signing.
				o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
				o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
				o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
					return stack.Finalize.Insert(dropAcceptEncoding, "Signing", middleware.Before)
				})
			}
		}),
	}, nil
}
