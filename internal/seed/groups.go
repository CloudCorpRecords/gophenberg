// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"

	"github.com/gopherium/gophenberg/internal/content"
)

// declareOnType declares the fields in the type's own field group, raising the group when none is held.
func declareOnType(
	ctx context.Context, types *content.Registry, typeKey string, fields ...content.Field,
) ([]content.Field, error) {
	group, err := groupOfType(ctx, types, typeKey)
	if err != nil {
		return nil, err
	}
	declared := make([]content.Field, 0, len(fields))
	for _, f := range fields {
		created, err := types.CreateFieldInGroup(ctx, group.ID, f)
		if err != nil {
			return nil, fmt.Errorf("seed %s field: %w", f.Key, err)
		}
		declared = append(declared, created)
	}
	return declared, nil
}

// groupOfType returns the type's own field group, raised when missing, or the reason the seed leaves it alone.
func groupOfType(ctx context.Context, types *content.Registry, typeKey string) (content.Group, error) {
	groups, err := types.Groups(ctx)
	if err != nil {
		return content.Group{}, fmt.Errorf("seed field group lookup: %w", err)
	}
	key := typeKey + "-fields"
	for _, g := range groups {
		if g.Key != key {
			continue
		}
		if err := servesType(ctx, types, g, typeKey); err != nil {
			return content.Group{}, err
		}
		return g, nil
	}
	return raiseGroup(ctx, types, key, typeKey)
}

// servesType returns the refusal for a group that is turned off or no longer reaches the type, or nothing.
func servesType(ctx context.Context, types *content.Registry, g content.Group, typeKey string) error {
	if !g.Active {
		return fmt.Errorf("seed %s group: it is turned off", g.Key)
	}
	if !g.Location.Match(content.Screen{content.ScreenContentType: typeKey}, types.Params(ctx)) {
		return fmt.Errorf("seed %s group: it no longer reaches the %s type", g.Key, typeKey)
	}
	return nil
}

// raiseGroup raises the type's own field group under the key, titled after the type and reaching it alone.
func raiseGroup(ctx context.Context, types *content.Registry, key, typeKey string) (content.Group, error) {
	named, err := types.ByKey(ctx, typeKey)
	if err != nil {
		return content.Group{}, fmt.Errorf("seed %s type lookup: %w", typeKey, err)
	}
	raised, err := types.CreateGroup(ctx, content.Group{
		Key: key, Title: named.SingularLabel + " fields",
		Location: content.Rules{{{Source: content.ScreenContentType, Operator: content.OperatorIs, Value: typeKey}}},
	})
	if err != nil {
		return content.Group{}, fmt.Errorf("seed %s group: %w", key, err)
	}
	return raised, nil
}
