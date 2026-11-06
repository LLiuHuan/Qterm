package store

import (
	"context"

	"changeme/internal/assetdata"
)

func (s *Store) ExportAssets(ctx context.Context) (assetdata.Snapshot, error) {
	data := assetdata.Snapshot{Version: 1}
	hosts, err := s.ListHosts(ctx)
	if err != nil {
		return data, err
	}
	for _, host := range hosts {

		item := assetdata.Host{Host: host}
		if host.PasswordConfigured {
			if err := s.UseHostPassword(ctx, host.ID, func(value []byte) error {
				item.Password = append([]byte(nil), value...)
				return nil
			}); err != nil {
				data.Clear()
				return data, err
			}
		}
		if host.ProxyPasswordConfigured {
			if err := s.UseHostProxyPassword(ctx, host.ID, func(value []byte) error {
				item.ProxyPassword = append([]byte(nil), value...)
				return nil
			}); err != nil {
				data.Clear()
				return data, err
			}
		}
		data.Hosts = append(data.Hosts, item)
	}
	data.Groups, err = s.ListGroups(ctx)
	if err != nil {
		data.Clear()
		return data, err
	}
	data.SnippetGroups, err = s.ListSnippetGroups(ctx)
	if err != nil {
		data.Clear()
		return data, err
	}

	for i := range data.SnippetGroups {
		var created, updated string
		if err = s.db.QueryRowContext(ctx, `SELECT created_at,updated_at FROM snippet_groups WHERE id=?`, data.SnippetGroups[i].ID).Scan(&created, &updated); err != nil {
			data.Clear()
			return data, err
		}
		data.SnippetGroups[i].CreatedAt, err = parseTime(created)
		if err != nil {
			data.Clear()
			return data, err
		}
		data.SnippetGroups[i].UpdatedAt, err = parseTime(updated)
		if err != nil {
			data.Clear()
			return data, err
		}
	}
	snippets, err := s.ListSnippets(ctx)
	if err != nil {
		data.Clear()
		return data, err
	}
	for _, snippet := range snippets {
		item := assetdata.Snippet{Snippet: snippet}
		if err := s.db.QueryRowContext(ctx, `SELECT sort_order FROM snippets WHERE id=?`, snippet.ID).Scan(&item.SortOrder); err != nil {
			data.Clear()
			return data, err
		}
		data.Snippets = append(data.Snippets, item)
	}

	data.CredentialPromptRules, err = s.ListCredentialPromptRules(ctx)
	if err != nil {
		data.Clear()
		return data, err
	}

	proxies, err := s.ListProxies(ctx)
	if err != nil {
		data.Clear()
		return data, err
	}
	for _, proxy := range proxies {
		item := assetdata.Proxy{
			ID: proxy.ID, Name: proxy.Name, Mode: proxy.Mode,
			Host: proxy.Host, Port: proxy.Port, Username: proxy.Username,
			CreatedAt: proxy.CreatedAt, UpdatedAt: proxy.UpdatedAt,
		}
		if proxy.PasswordConfigured {
			if err := s.UseProxyPassword(ctx, proxy.ID, func(value []byte) error {
				item.Password = append([]byte(nil), value...)
				return nil
			}); err != nil {
				data.Clear()
				return data, err
			}
		}
		data.Proxies = append(data.Proxies, item)
	}
	return data, nil
}
