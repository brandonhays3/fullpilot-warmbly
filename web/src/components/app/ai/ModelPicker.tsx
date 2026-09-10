// ModelPicker: a searchable dropdown over OpenRouter's model catalog, fed by
// GET /organization/current/ai/models. Search matches id and name; each row
// shows the prompt / completion price per million tokens so the choice is an
// informed one. Pass `defaultLabel` to offer "Workspace default" as the empty
// value (an AI block that follows Settings > AI).

import React from "react";
import { ChevronDownIcon, CheckIcon, Loader2Icon } from "@/components/icons";
import { SearchInput } from "@/components/ui/field";
import { PopoverMenu, PopoverMenuContent, PopoverMenuTrigger } from "@/components/ui/popover-menu";
import { type AIModel, formatModelPrice } from "@/lib/api/models/app/organizations/AISettings";
import { cn } from "@/lib/utils";

// Rows rendered at once. The catalog runs to several hundred entries; search
// narrows it long before anyone scrolls that far.
const MAX_ROWS = 150;

export default function ModelPicker({
    value,
    onChange,
    models,
    loading = false,
    disabled = false,
    defaultLabel,
    placeholder = "Pick a model",
    fullWidth = false,
    className,
}: {
    value: string;
    onChange: (id: string) => void;
    models: AIModel[];
    loading?: boolean;
    disabled?: boolean;
    // When set, an extra first option with value "" reads "Workspace default
    // (<defaultLabel>)".
    defaultLabel?: string;
    placeholder?: string;
    fullWidth?: boolean;
    className?: string;
}) {
    const [open, setOpen] = React.useState(false);
    const [query, setQuery] = React.useState("");

    const byId = React.useMemo(() => new Map(models.map((m) => [m.id, m])), [models]);
    const current = value ? byId.get(value) : undefined;

    const filtered = React.useMemo(() => {
        const q = query.trim().toLowerCase();
        if (!q) return models;
        const terms = q.split(/\s+/);
        return models.filter((m) => {
            const hay = `${m.id} ${m.name}`.toLowerCase();
            return terms.every((t) => hay.includes(t));
        });
    }, [models, query]);

    const shown = filtered.slice(0, MAX_ROWS);
    const hidden = filtered.length - shown.length;

    let label: string;
    if (value) label = current?.name ?? value;
    else if (defaultLabel !== undefined) label = `Workspace default (${defaultLabel})`;
    else label = placeholder;

    return (
        <PopoverMenu
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) setQuery("");
            }}
        >
            <PopoverMenuTrigger asChild>
                <button
                    type="button"
                    disabled={disabled}
                    className={cn(
                        "h-7 px-2.5 border border-slate-200 hover:border-slate-300 bg-white text-[12px] text-slate-700 items-center gap-1.5 transition-colors disabled:opacity-60 disabled:cursor-not-allowed",
                        fullWidth ? "flex w-full" : "inline-flex max-w-full",
                        className,
                    )}
                >
                    <span className={cn("truncate flex-1 text-left", !value && defaultLabel === undefined && "text-slate-400")}>
                        {label}
                    </span>
                    {loading ? (
                        <Loader2Icon className="w-3 h-3 text-slate-400 shrink-0 animate-spin" />
                    ) : (
                        <ChevronDownIcon className="w-3 h-3 text-slate-400 shrink-0" />
                    )}
                </button>
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={fullWidth ? 0 : 360} matchTriggerWidth={fullWidth} className="p-0 max-h-[24rem] flex flex-col">
                <div className="shrink-0 p-2 border-b border-slate-200">
                    <SearchInput value={query} onChange={setQuery} placeholder="Search models by id or name" autoFocus />
                </div>
                <div className="min-h-0 flex-1 overflow-y-auto py-1">
                    {defaultLabel !== undefined && !query && (
                        <ModelRow
                            id=""
                            name="Workspace default"
                            hint={defaultLabel}
                            selected={!value}
                            onSelect={() => {
                                onChange("");
                                setOpen(false);
                            }}
                        />
                    )}
                    {loading && models.length === 0 ? (
                        <p className="px-3 py-3 text-[12px] text-slate-500">Loading OpenRouter's model list…</p>
                    ) : shown.length === 0 ? (
                        <p className="px-3 py-3 text-[12px] text-slate-500">
                            {models.length === 0 ? "No models available." : "No model matches that search."}
                        </p>
                    ) : (
                        shown.map((m) => (
                            <ModelRow
                                key={m.id}
                                id={m.id}
                                name={m.name}
                                price={`${formatModelPrice(m.prompt_per_million)} / ${formatModelPrice(m.completion_per_million)}`}
                                selected={m.id === value}
                                onSelect={() => {
                                    onChange(m.id);
                                    setOpen(false);
                                }}
                            />
                        ))
                    )}
                    {hidden > 0 && (
                        <p className="px-3 py-2 text-[11px] text-slate-400">
                            {hidden} more. Keep typing to narrow the list.
                        </p>
                    )}
                </div>
                <div className="shrink-0 px-3 py-1.5 border-t border-slate-200 text-[10.5px] text-slate-400">
                    Prices are per million tokens, prompt / completion, billed by OpenRouter.
                </div>
            </PopoverMenuContent>
        </PopoverMenu>
    );
}

function ModelRow({
    id,
    name,
    hint,
    price,
    selected,
    onSelect,
}: {
    id: string;
    name: string;
    hint?: string;
    price?: string;
    selected: boolean;
    onSelect: () => void;
}) {
    return (
        <button
            type="button"
            role="menuitem"
            onClick={(e) => {
                e.stopPropagation();
                onSelect();
            }}
            className={cn(
                "w-full px-3 py-1.5 flex items-center gap-3 text-left transition-colors hover:bg-slate-50",
                selected && "bg-sky-50",
            )}
        >
            <span className="min-w-0 flex-1">
                <span className={cn("block truncate text-[12.5px] text-slate-800", selected && "font-medium text-slate-900")}>
                    {name}
                </span>
                <span className="block truncate text-[11px] text-slate-400">{hint ?? id}</span>
            </span>
            {price && <span className="shrink-0 text-[11px] text-slate-500 tabular-nums">{price}</span>}
            {selected && <CheckIcon className="w-3.5 h-3.5 text-sky-600 shrink-0" />}
        </button>
    );
}
