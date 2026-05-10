package tui

// This file consolidates the previously-duplicated widget-section logic that
// lived twice in model.go: once for dashboard widgets and once for detail
// widgets. The two flows shared structure: normalize input, build a default
// list, and resolve the user-provided list against the default by appending
// missing entries. They differed in the section ID type, in whether the
// "header" section was excluded, and in which cache to invalidate.
//
// The generic mergeSections + normalizeSections functions accept a tiny
// trait-style struct describing those differences. The dashboard and detail
// callers both fit through the same helpers; the wrappers in model.go now
// pass through trait values and add only the cache-invalidation hook.

// sectionTrait describes how a particular section family normalises and
// orders its entries. ID is the section identifier type (e.g.
// core.DashboardStandardSection); Section is the persisted entry type
// (e.g. config.DashboardWidgetSection).
type sectionTrait[ID comparable, Section any] struct {
	// extractID returns the ID from a section entry.
	extractID func(Section) ID
	// extractEnabled returns the enabled flag from a section entry.
	extractEnabled func(Section) bool
	// build constructs a new section entry from an (ID, enabled) pair.
	build func(ID, bool) Section
	// normalizeID lower-cases / aliases the ID before comparison.
	normalizeID func(ID) ID
	// keepID returns true when this ID should appear in normalized output.
	// (For dashboard, this excludes the "header" section and unknown IDs.
	// For detail, it just gates on known-ness.)
	keepID func(ID) bool
	// defaultIDs returns the canonical default order for this section family.
	defaultIDs func() []ID
}

// normalizeSections drops blank/unknown/duplicate entries and produces a
// stable rebuild of the user's intent — the canonical form we persist.
func normalizeSections[ID comparable, Section any](
	entries []Section, t sectionTrait[ID, Section],
) []Section {
	if len(entries) == 0 {
		return nil
	}
	out := make([]Section, 0, len(entries))
	seen := make(map[ID]bool, len(entries))
	for _, entry := range entries {
		id := t.normalizeID(t.extractID(entry))
		if !t.keepID(id) || seen[id] {
			continue
		}
		out = append(out, t.build(id, t.extractEnabled(entry)))
		seen[id] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// defaultSections returns one entry per default ID with Enabled=true.
func defaultSections[ID comparable, Section any](t sectionTrait[ID, Section]) []Section {
	ids := t.defaultIDs()
	out := make([]Section, 0, len(ids))
	for _, id := range ids {
		out = append(out, t.build(id, true))
	}
	return out
}

// mergeSections returns the user's entries followed by any default entries
// the user didn't include — keeping the user's ordering for known sections
// and appending newcomers in their canonical order.
func mergeSections[ID comparable, Section any](
	user []Section, t sectionTrait[ID, Section],
) []Section {
	if len(user) == 0 {
		return defaultSections(t)
	}

	out := make([]Section, len(user))
	copy(out, user)

	seen := make(map[ID]bool, len(out))
	for _, entry := range out {
		seen[t.extractID(entry)] = true
	}
	for _, entry := range defaultSections(t) {
		if seen[t.extractID(entry)] {
			continue
		}
		out = append(out, entry)
	}
	return out
}
