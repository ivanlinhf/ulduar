const placeholderChats = [
  {
    id: "current-chat",
    title: "Current chat",
    preview: "This layout preview keeps the active conversation on the right.",
    meta: "Available now",
    isCurrent: true,
  },
  {
    id: "release-notes",
    title: "Release planning notes",
    preview: "Session restore will arrive in a future update.",
    meta: "Coming soon",
  },
  {
    id: "attachment-review",
    title: "Attachment review",
    preview: "Image and PDF conversations will appear here later.",
    meta: "Preview only",
  },
  {
    id: "design-pass",
    title: "Design feedback draft",
    preview: "Placeholder rows show how session items will be arranged.",
    meta: "Coming soon",
  },
  {
    id: "support-followup",
    title: "Support follow-up",
    preview: "Browsing older chats is not wired up in this release.",
    meta: "Preview only",
  },
  {
    id: "weekly-recap",
    title: "Weekly recap",
    preview: "Delete, paging, and filtering stay unavailable for now.",
    meta: "Coming soon",
  },
];

export function ChatHistorySidebar() {
  return (
    <aside aria-label="Chat history preview" className="history-sidebar">
      <div className="history-sidebar-header">
        <p className="eyebrow">Chats</p>
        <h2 className="history-sidebar-title">History</h2>
        <p className="history-sidebar-note">
          History will appear here soon. Session restore is not available in this release.
        </p>
      </div>

      <div className="history-sidebar-search">
        <input
          aria-label="Search chats"
          className="history-search-input"
          disabled
          placeholder="Search history (coming soon)"
          type="search"
        />
        <button className="history-filter-button" disabled type="button">
          Filters unavailable
        </button>
      </div>

      <ul className="history-sidebar-list">
        {placeholderChats.map((chat) => (
          <li className="history-row" data-current={chat.isCurrent ? "true" : undefined} key={chat.id}>
            <div className="history-row-heading">
              <strong>{chat.title}</strong>
              <span className="history-row-meta">{chat.meta}</span>
            </div>
            <p>{chat.preview}</p>
          </li>
        ))}
      </ul>

      <p className="history-sidebar-footer">
        Session browsing, paging, and delete are not available yet.
      </p>
    </aside>
  );
}
