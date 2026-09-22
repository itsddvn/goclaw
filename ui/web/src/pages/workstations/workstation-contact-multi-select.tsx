import { useId, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown, Loader2, X } from "lucide-react";
import { usePortalDropdownClose } from "@/hooks/use-portal-dropdown-close";
import { cn } from "@/lib/utils";
import type { ChannelContact } from "@/types/contact";

interface WorkstationContactMultiSelectProps {
  contacts: ChannelContact[];
  selectedContacts: ChannelContact[];
  search: string;
  onSearchChange: (search: string) => void;
  onToggle: (contact: ChannelContact) => void;
  placeholder: string;
  loadingText: string;
  noResultsText: string;
  removeSelectionLabel: (contact: string) => string;
  disabled?: boolean;
  loading?: boolean;
}

function contactName(contact: ChannelContact): string {
  return contact.display_name || contact.username || contact.sender_id;
}

function contactDetail(contact: ChannelContact): string {
  const username = contact.username && contact.username !== contactName(contact)
    ? ` @${contact.username}`
    : "";
  return `${contactName(contact)}${username} · ${contact.channel_type} · ${contact.sender_id}`;
}

export function WorkstationContactMultiSelect({
  contacts,
  selectedContacts,
  search,
  onSearchChange,
  onToggle,
  placeholder,
  loadingText,
  noResultsText,
  removeSelectionLabel,
  disabled,
  loading,
}: WorkstationContactMultiSelectProps) {
  const [open, setOpen] = useState(false);
  const [dropdownStyle, setDropdownStyle] = useState<React.CSSProperties>({});
  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const listboxId = useId();
  const selectedIds = useMemo(
    () => new Set(selectedContacts.map((contact) => contact.id)),
    [selectedContacts],
  );

  usePortalDropdownClose({
    open,
    onClose: () => setOpen(false),
    ignore: [containerRef, dropdownRef],
  });

  useLayoutEffect(() => {
    if (!open || !containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    const gap = 4;
    const maxHeight = 256;
    const flipUp = window.innerHeight - rect.bottom < maxHeight && rect.top > maxHeight;
    setDropdownStyle(
      flipUp
        ? {
            position: "fixed",
            bottom: window.innerHeight - rect.top + gap,
            left: rect.left,
            width: rect.width,
            maxHeight,
            zIndex: 9999,
          }
        : {
            position: "fixed",
            top: rect.bottom + gap,
            left: rect.left,
            width: rect.width,
            maxHeight,
            zIndex: 9999,
          },
    );
  }, [contacts.length, loading, open, search, selectedContacts.length]);

  function toggleContact(contact: ChannelContact) {
    onToggle(contact);
    setOpen(true);
    requestAnimationFrame(() => inputRef.current?.focus());
  }

  return (
    <div ref={containerRef} className="relative min-w-0 flex-1">
      <div
        className={cn(
          "border-input dark:bg-input/30 flex min-h-9 flex-wrap items-center gap-1 rounded-md border bg-transparent px-2 py-1 shadow-xs transition-[color,box-shadow]",
          "focus-within:border-ring focus-within:ring-ring/50 focus-within:ring-1",
          disabled && "cursor-not-allowed opacity-50",
        )}
        onClick={() => !disabled && inputRef.current?.focus()}
      >
        {selectedContacts.map((contact) => {
          const name = contactName(contact);
          return (
            <span
              key={contact.id}
              className="bg-secondary text-secondary-foreground inline-flex max-w-full items-center gap-1 rounded px-1.5 py-0.5 text-xs"
            >
              <span className="max-w-44 truncate">{name} · {contact.channel_type}</span>
              <button
                type="button"
                disabled={disabled}
                aria-label={removeSelectionLabel(name)}
                className="hover:text-destructive relative ml-0.5 cursor-pointer rounded-full p-0.5 after:absolute after:-inset-2 after:content-[''] md:after:hidden"
                onClick={(event) => {
                  event.stopPropagation();
                  toggleContact(contact);
                }}
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          );
        })}
        <input
          ref={inputRef}
          role="combobox"
          aria-expanded={open}
          aria-controls={listboxId}
          aria-autocomplete="list"
          value={search}
          disabled={disabled}
          placeholder={selectedContacts.length === 0 ? placeholder : ""}
          className="placeholder:text-muted-foreground min-w-32 flex-1 bg-transparent py-0.5 text-base outline-none md:text-sm"
          onFocus={() => setOpen(true)}
          onChange={(event) => {
            onSearchChange(event.target.value);
            setOpen(true);
          }}
          onKeyDown={(event) => {
            if (event.key === "Escape") setOpen(false);
            if (event.key === "Backspace" && !search && selectedContacts.length > 0) {
              toggleContact(selectedContacts[selectedContacts.length - 1]!);
            }
          }}
        />
        {loading && <Loader2 className="text-muted-foreground size-4 shrink-0 animate-spin" aria-hidden />}
        <button
          type="button"
          disabled={disabled}
          aria-label={placeholder}
          aria-expanded={open}
          className="text-muted-foreground relative flex size-7 shrink-0 items-center justify-center rounded opacity-60 hover:bg-accent"
          onClick={(event) => {
            event.stopPropagation();
            setOpen((current) => !current);
          }}
        >
          <ChevronDown className="size-4" />
        </button>
      </div>

      {open && createPortal(
        <div
          ref={dropdownRef}
          id={listboxId}
          role="listbox"
          aria-multiselectable="true"
          style={dropdownStyle}
          className="bg-popover text-popover-foreground pointer-events-auto overflow-y-auto rounded-md border p-1 shadow-md"
        >
          {loading && (
            <div className="text-muted-foreground flex items-center gap-2 px-2 py-2 text-xs">
              <Loader2 className="size-3.5 animate-spin" aria-hidden />
              {loadingText}
            </div>
          )}
          {!loading && contacts.length === 0 && (
            <p className="text-muted-foreground px-2 py-3 text-center text-xs">{noResultsText}</p>
          )}
          {contacts.map((contact) => {
            const selected = selectedIds.has(contact.id);
            return (
              <button
                key={contact.id}
                type="button"
                role="option"
                aria-selected={selected}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => toggleContact(contact)}
                className={cn(
                  "hover:bg-accent hover:text-accent-foreground flex w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-2 text-left outline-hidden select-none",
                  selected && "bg-accent/60",
                )}
              >
                <span className={cn(
                  "border-input flex size-4 shrink-0 items-center justify-center rounded border",
                  selected && "border-primary bg-primary text-primary-foreground",
                )}>
                  {selected && <Check className="size-3" aria-hidden />}
                </span>
                <span className="min-w-0 flex-1 truncate text-sm">{contactDetail(contact)}</span>
              </button>
            );
          })}
        </div>,
        document.body,
      )}
    </div>
  );
}
