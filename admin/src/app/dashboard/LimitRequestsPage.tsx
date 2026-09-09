// Limit-increase request queue — left filter rail + server-driven sortable,
// cursor-paged table (mirrors OrganizationsPage). Approving writes the
// corresponding override on the org via the same SetLimitOverrides path the
// manual editor uses; rejecting stamps the row with required review notes.

import { useEffect, useState } from "react";
import {
    keepPreviousData,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { CheckCircle2, XCircle } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    Explorer,
    FilterGroup,
    SearchFilter,
    SelectFilter,
    ToggleFilter,
    DateRangeFilter,
    NumberRangeFilter,
} from "@/components/data/Explorer";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useCursorPager } from "@/lib/useCursorPager";
import {
    emptyRange,
    rangeActive,
    rangeWithin,
    rangeAfter,
    rangeBefore,
    type DateRange,
} from "@/lib/dateRange";
import {
    approveLimitRequest,
    listLimitRequests,
    rejectLimitRequest,
} from "@/lib/api/client/admin/limitRequests";
import type {
    AdminLimitRequestSearch,
    LimitIncreaseRequest,
    LimitRequestStatus,
} from "@/lib/api/models/admin";

type StatusFilter = LimitRequestStatus | "all";

const STATUS_TONE: Record<LimitRequestStatus, string> = {
    pending: "border-amber-300 text-amber-700 bg-amber-50",
    approved: "border-emerald-300 text-emerald-700 bg-emerald-50",
    rejected: "border-red-300 text-red-700 bg-red-50",
    cancelled: "border-zinc-300 text-zinc-600 bg-zinc-50",
};

const FIELD_LABEL: Record<string, string> = {
    max_email_accounts: "Mailboxes",
    max_campaigns: "Campaigns (lifetime)",
    max_active_campaigns: "Active campaigns",
    max_team_members: "Team members",
    max_contacts: "Contacts",
    daily_campaign_limit: "Daily sends",
};

const STATUS_OPTIONS = [
    { value: "any", label: "Any status" },
    { value: "pending", label: "Pending" },
    { value: "approved", label: "Approved" },
    { value: "rejected", label: "Rejected" },
    { value: "cancelled", label: "Cancelled" },
];

const FIELD_OPTIONS = [
    { value: "any", label: "Any field" },
    ...Object.entries(FIELD_LABEL).map(([value, label]) => ({ value, label })),
];

export default function LimitRequestsPage() {
    const nav = useNavigate();
    const [query, setQuery] = useState("");
    const [status, setStatus] = useState<StatusFilter>("pending");
    const [field, setField] = useState("");
    const [reviewed, setReviewed] = useState(false);
    const [unreviewed, setUnreviewed] = useState(false);
    const [reqMin, setReqMin] = useState<number | undefined>();
    const [reqMax, setReqMax] = useState<number | undefined>();
    const [curMin, setCurMin] = useState<number | undefined>();
    const [curMax, setCurMax] = useState<number | undefined>();
    const [submitted, setSubmitted] = useState<DateRange>(emptyRange);
    const [reviewedRange, setReviewedRange] = useState<DateRange>(emptyRange);

    const [sort, setSort] = useState<{ by: string; desc: boolean }>({ by: "", desc: true });
    const pager = useCursorPager();
    const { reset } = pager;

    const [reviewing, setReviewing] = useState<{
        req: LimitIncreaseRequest;
        mode: "approve" | "reject";
    } | null>(null);

    const filterKey = JSON.stringify({
        query, status, field, reviewed, unreviewed,
        reqMin, reqMax, curMin, curMax, submitted, reviewedRange, sort,
    });

    useEffect(() => {
        reset();
    }, [filterKey, reset]);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "limit-requests", filterKey, pager.cursor],
        queryFn: () =>
            listLimitRequests({
                q: query.trim() || undefined,
                status: status === "all" ? "" : status,
                field: field || undefined,
                reviewed: reviewed || undefined,
                unreviewed: unreviewed || undefined,
                requested_min: reqMin,
                requested_max: reqMax,
                current_effective_min: curMin,
                current_effective_max: curMax,
                submitted_within: rangeWithin(submitted),
                submitted_after: rangeAfter(submitted),
                submitted_before: rangeBefore(submitted),
                reviewed_after: rangeAfter(reviewedRange),
                reviewed_before: rangeBefore(reviewedRange),
                limit: 50,
                cursor: pager.cursor,
                sort_by: sort.by ? (sort.by as AdminLimitRequestSearch["sort_by"]) : undefined,
                sort_desc: sort.by ? sort.desc : undefined,
            }),
        staleTime: 30_000,
        placeholderData: keepPreviousData,
    });

    const rows = data?.data ?? [];

    const bools = [reviewed, unreviewed];
    const ranges = [[reqMin, reqMax], [curMin, curMax]];
    const activeCount =
        (query ? 1 : 0) +
        (status !== "pending" ? 1 : 0) +
        (field ? 1 : 0) +
        bools.filter(Boolean).length +
        ranges.filter(([a, b]) => a !== undefined || b !== undefined).length +
        [submitted, reviewedRange].filter(rangeActive).length +
        (sort.by ? 1 : 0);

    function resetAll() {
        setQuery("");
        setStatus("pending");
        setField("");
        setReviewed(false);
        setUnreviewed(false);
        setReqMin(undefined);
        setReqMax(undefined);
        setCurMin(undefined);
        setCurMax(undefined);
        setSubmitted(emptyRange);
        setReviewedRange(emptyRange);
        setSort({ by: "", desc: true });
    }

    const columns: Column<LimitIncreaseRequest>[] = [
        {
            id: "workspace",
            header: "Workspace",
            sortable: true,
            sortKey: "org_name",
            cell: (r) => (
                <div>
                    <Link
                        to={`/organizations/${r.organization_id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="font-medium text-[var(--admin-accent-strong)] hover:underline"
                    >
                        {r.organization?.name ?? r.organization_id}
                    </Link>
                    {r.organization?.slug && (
                        <div className="font-mono text-[10px] text-muted-foreground">{r.organization.slug}</div>
                    )}
                </div>
            ),
            csv: (r) => r.organization?.name ?? r.organization_id,
        },
        {
            id: "requester",
            header: "Requester",
            cell: (r) => (
                <span className="text-xs">{r.submitted_by_user?.email ?? r.submitted_by}</span>
            ),
            csv: (r) => r.submitted_by_user?.email ?? r.submitted_by,
        },
        {
            id: "field",
            header: "Field",
            sortable: true,
            sortKey: "field",
            cell: (r) => <span className="text-xs">{FIELD_LABEL[r.field] ?? r.field}</span>,
            csv: (r) => FIELD_LABEL[r.field] ?? r.field,
        },
        {
            id: "current",
            header: "Current",
            align: "right",
            sortable: true,
            sortKey: "current_effective",
            cell: (r) => (
                <span className="tabular-nums text-muted-foreground">{r.current_effective.toLocaleString()}</span>
            ),
            csv: (r) => r.current_effective,
        },
        {
            id: "requested",
            header: "Requested",
            align: "right",
            sortable: true,
            sortKey: "requested",
            cell: (r) => (
                <span className="tabular-nums font-medium">
                    {r.requested.toLocaleString()}
                    <span className="text-[10px] text-emerald-600 ml-1">
                        (+{(r.requested - r.current_effective).toLocaleString()})
                    </span>
                </span>
            ),
            csv: (r) => r.requested,
        },
        {
            id: "reason",
            header: "Reason",
            cell: (r) => (
                <span className="text-xs max-w-md truncate block" title={r.reason}>
                    {r.reason}
                </span>
            ),
            csv: (r) => r.reason,
        },
        {
            id: "status",
            header: "Status",
            sortable: true,
            sortKey: "status",
            cell: (r) => (
                <div>
                    <Badge variant="outline" className={`text-[10px] ${STATUS_TONE[r.status]}`}>
                        {r.status}
                    </Badge>
                    {r.review_notes && r.status !== "pending" && (
                        <div className="text-[10px] text-muted-foreground mt-1 max-w-xs truncate" title={r.review_notes}>
                            "{r.review_notes}"
                        </div>
                    )}
                </div>
            ),
            csv: (r) => r.status,
        },
        {
            id: "submitted",
            header: "Submitted",
            sortable: true,
            sortKey: "submitted_at",
            cell: (r) => (
                <span className="text-xs text-muted-foreground">{new Date(r.submitted_at).toLocaleDateString()}</span>
            ),
            csv: (r) => r.submitted_at,
        },
        {
            id: "reviewed",
            header: "Reviewed",
            sortable: true,
            sortKey: "reviewed_at",
            defaultHidden: true,
            cell: (r) => (
                <span className="text-xs text-muted-foreground">
                    {r.reviewed_at ? new Date(r.reviewed_at).toLocaleDateString() : "—"}
                </span>
            ),
            csv: (r) => r.reviewed_at ?? "",
        },
        {
            id: "actions",
            header: "",
            align: "right",
            cell: (r) => {
                const canReview = r.status === "pending";
                return (
                    <div className="space-x-1.5 whitespace-nowrap">
                        <Button
                            size="sm"
                            disabled={!canReview}
                            onClick={(e) => {
                                e.stopPropagation();
                                setReviewing({ req: r, mode: "approve" });
                            }}
                            className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs disabled:bg-zinc-200"
                        >
                            <CheckCircle2 className="size-3" /> Approve
                        </Button>
                        <Button
                            size="sm"
                            disabled={!canReview}
                            onClick={(e) => {
                                e.stopPropagation();
                                setReviewing({ req: r, mode: "reject" });
                            }}
                            className="bg-red-600 hover:bg-red-700 text-white text-xs disabled:bg-zinc-200"
                        >
                            <XCircle className="size-3" /> Reject
                        </Button>
                    </div>
                );
            },
        },
    ];

    return (
        <div>
            <PageHeader
                title="Limit-increase requests"
                description="Customer-submitted requests for more capacity than their plan or product hard cap allows. Approving rewrites the per-org override; rejecting stamps the row with notes."
            />
            <Explorer
                activeCount={activeCount}
                onReset={resetAll}
                filters={
                    <>
                        <FilterGroup label="Search">
                            <SearchFilter value={query} onChange={setQuery} placeholder="Org, requester, or reason…" />
                        </FilterGroup>
                        <FilterGroup label="Status">
                            <SelectFilter
                                value={status === "all" ? "any" : status}
                                onChange={(v) => setStatus(v === "any" ? "all" : (v as LimitRequestStatus))}
                                options={STATUS_OPTIONS}
                                placeholder="Any status"
                            />
                        </FilterGroup>
                        <FilterGroup label="Field">
                            <SelectFilter
                                value={field || "any"}
                                onChange={(v) => setField(v === "any" ? "" : v)}
                                options={FIELD_OPTIONS}
                                placeholder="Any field"
                            />
                        </FilterGroup>
                        <FilterGroup label="Review state">
                            <div className="flex flex-col gap-2">
                                <ToggleFilter checked={reviewed} onChange={setReviewed} label="Reviewed" />
                                <ToggleFilter checked={unreviewed} onChange={setUnreviewed} label="Awaiting review" />
                            </div>
                        </FilterGroup>
                        <FilterGroup label="Requested amount">
                            <NumberRangeFilter min={reqMin} max={reqMax} onMinChange={setReqMin} onMaxChange={setReqMax} />
                        </FilterGroup>
                        <FilterGroup label="Current effective">
                            <NumberRangeFilter min={curMin} max={curMax} onMinChange={setCurMin} onMaxChange={setCurMax} />
                        </FilterGroup>
                        <FilterGroup label="Submitted">
                            <DateRangeFilter value={submitted} onChange={setSubmitted} />
                        </FilterGroup>
                        <FilterGroup label="Reviewed">
                            <DateRangeFilter value={reviewedRange} onChange={setReviewedRange} mode="custom" />
                        </FilterGroup>
                    </>
                }
            >
                <DataTable
                    columns={columns}
                    rows={rows}
                    getRowId={(r) => r.id}
                    loading={isLoading}
                    error={error}
                    onRetry={() => refetch()}
                    errorTitle="Failed to load limit requests"
                    onRowClick={(r) => nav(`/organizations/${r.organization_id}`)}
                    sort={sort.by ? sort : undefined}
                    onSortChange={setSort}
                    storageKey="admin.limit-requests"
                    csvName="fullpilot-limit-requests"
                    noun="requests"
                    emptyTitle="No limit requests"
                    emptyHint="No requests match these filters."
                    pager={{
                        canPrev: pager.canPrev,
                        canNext: !!data?.pagination?.has_more,
                        onPrev: pager.prev,
                        onNext: () => pager.next(data?.pagination?.next_cursor),
                        page: pager.page,
                        shown: rows.length,
                        total: data?.pagination?.total ?? null,
                    }}
                />
            </Explorer>

            {reviewing && (
                <ReviewDialog
                    req={reviewing.req}
                    mode={reviewing.mode}
                    open
                    onOpenChange={(v) => !v && setReviewing(null)}
                />
            )}
        </div>
    );
}

function ReviewDialog({
    req,
    mode,
    open,
    onOpenChange,
}: {
    req: LimitIncreaseRequest;
    mode: "approve" | "reject";
    open: boolean;
    onOpenChange: (v: boolean) => void;
}) {
    const qc = useQueryClient();
    const [notes, setNotes] = useState("");
    const mutation = useMutation({
        mutationFn: () =>
            mode === "approve"
                ? approveLimitRequest(req.id, notes)
                : rejectLimitRequest(req.id, notes),
        onSuccess: () => {
            toast.success(`Request ${mode === "approve" ? "approved" : "rejected"}`);
            qc.invalidateQueries({ queryKey: ["admin", "limit-requests"] });
            qc.invalidateQueries({ queryKey: ["admin", "organizations", req.organization_id] });
            onOpenChange(false);
        },
        onError: (err: Error) => toast.error(err.message || "Action failed"),
    });

    const fieldLabel = FIELD_LABEL[req.field] ?? req.field;

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {mode === "approve" ? "Approve request" : "Reject request"}
                    </DialogTitle>
                    <DialogDescription>
                        {mode === "approve" ? (
                            <>
                                Approving raises <strong>{fieldLabel}</strong> for{" "}
                                <span className="font-mono">
                                    {req.organization?.name ?? req.organization_id}
                                </span>{" "}
                                from {req.current_effective.toLocaleString()} to{" "}
                                {req.requested.toLocaleString()}. This writes the
                                corresponding override on the org and is auditable.
                            </>
                        ) : (
                            <>
                                Rejecting the request stamps it with your notes for the
                                customer to see. The org keeps its current effective
                                limit ({req.current_effective.toLocaleString()}).
                            </>
                        )}
                    </DialogDescription>
                </DialogHeader>
                <div>
                    <Label htmlFor="notes" className="text-xs font-medium">
                        Notes {mode === "reject" ? "(required)" : "(optional)"}
                    </Label>
                    <Input
                        id="notes"
                        placeholder={
                            mode === "approve"
                                ? "Optional: business reason for the bump"
                                : "Required: tell the customer why"
                        }
                        value={notes}
                        onChange={(e) => setNotes(e.target.value)}
                        autoFocus
                    />
                </div>
                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button
                        onClick={() => {
                            if (mode === "reject" && notes.trim() === "") {
                                toast.error("Notes are required when rejecting");
                                return;
                            }
                            mutation.mutate();
                        }}
                        disabled={mutation.isPending}
                        className={
                            mode === "approve"
                                ? "bg-emerald-600 hover:bg-emerald-700 text-white"
                                : "bg-red-600 hover:bg-red-700 text-white"
                        }
                    >
                        {mutation.isPending
                            ? "Working…"
                            : mode === "approve"
                            ? "Approve"
                            : "Reject"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
