"use client";

import { useState } from "react";
import {
  useRedactConfig,
  useUpdateRedactConfig,
} from "@/lib/queries";
import { api } from "@/lib/api";
import type { RedactConfig, RedactRule } from "@/lib/types";
import { Panel, PanelHeader, PanelTitle, PanelBody } from "@/components/ui/panel";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { EyeOff, Plus, Trash2, Save, Wand2 } from "lucide-react";

export function PrivacyPanel() {
  const { data: initial, isLoading } = useRedactConfig();
  const updateMut = useUpdateRedactConfig();
  const [draft, setDraft] = useState<RedactConfig | null>(null);
  const [newWord, setNewWord] = useState("");
  const [newRuleKind, setNewRuleKind] = useState("");
  const [newRulePrefix, setNewRulePrefix] = useState("");
  const [newRuleRegex, setNewRuleRegex] = useState("");
  const [previewIn, setPreviewIn] = useState("");
  const [previewOut, setPreviewOut] = useState("");

  const config = draft ?? initial;
  const isDirty = draft != null;

  if (isLoading || !config) {
    return <Skeleton className="h-64" />;
  }

  function set<K extends keyof RedactConfig>(key: K, value: RedactConfig[K]) {
    setDraft((prev) => ({ ...(prev ?? initial ?? ({} as RedactConfig)), [key]: value }));
  }

  async function save() {
    if (!config) return;
    try {
      await updateMut.mutateAsync(config);
      toast.success("masking settings applied", {
        description: "Live for new /v1 requests; persisted to config.yaml",
      });
      setDraft(null);
    } catch (err) {
      toast.error("save failed", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  function addWord() {
    const w = newWord.trim();
    if (!w) return;
    set("words", [...(draft?.words ?? initial?.words ?? []), w]);
    setNewWord("");
  }

  function addRule() {
    const rule: RedactRule = { kind: newRuleKind.trim() || "CUSTOM" };
    if (newRuleRegex.trim()) {
      rule.regex = newRuleRegex.trim();
    } else if (newRulePrefix.trim()) {
      rule.prefix = newRulePrefix.trim();
    } else {
      toast.error("a rule needs a prefix or a regex");
      return;
    }
    if (rule.prefix && rule.regex) {
      toast.error("a rule has a prefix or a regex, not both");
      return;
    }
    set("rules", [...(draft?.rules ?? initial?.rules ?? []), rule]);
    setNewRuleKind("");
    setNewRulePrefix("");
    setNewRuleRegex("");
  }

  async function runPreview() {
    if (!previewIn.trim()) return;
    try {
      const res = await api.previewRedact({ text: previewIn, config: draft ?? undefined });
      setPreviewOut(res.masked);
    } catch (err) {
      toast.error("preview failed", {
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }

  return (
    <Panel>
      <PanelHeader>
        <div className="flex items-center gap-2">
          <EyeOff className="h-3.5 w-3.5 text-primary" />
          <PanelTitle>privacy masking</PanelTitle>
        </div>
        <div className="flex items-center gap-2">
          {isDirty && <Badge variant="accent">unsaved</Badge>}
          <Badge variant={config.enabled ? "success" : "outline"}>
            {config.enabled ? "on" : "off"}
          </Badge>
          <Button size="sm" onClick={save} disabled={!isDirty || updateMut.isPending}>
            <Save className="h-3.5 w-3.5" />
            apply
          </Button>
        </div>
      </PanelHeader>
      <PanelBody className="space-y-5">
        <p className="text-[12px] text-muted-foreground">
          Sensitive values found in /v1 request bodies go to the provider as stable
          placeholders ({"{{"}KIND_tag{"}}"}) and are put back in the response. Same
          value → same placeholder, so conversations and prompt caches stay valid.
        </p>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div className="flex items-center justify-between rounded-sm border border-border px-3 py-2.5">
            <div>
              <div className="text-[13px]">masking on</div>
              <div className="text-[11px] text-muted-foreground">master switch for /v1 requests</div>
            </div>
            <Switch
              checked={config.enabled}
              onCheckedChange={(v) => set("enabled", v)}
            />
          </div>
          <div className="flex items-center justify-between rounded-sm border border-border px-3 py-2.5">
            <div>
              <div className="text-[13px]">secrets</div>
              <div className="text-[11px] text-muted-foreground">API keys, tokens, passwords, private keys</div>
            </div>
            <Switch
              checked={config.secrets}
              onCheckedChange={(v) => set("secrets", v)}
            />
          </div>
          <div className="flex items-center justify-between rounded-sm border border-border px-3 py-2.5">
            <div>
              <div className="text-[13px]">personal data</div>
              <div className="text-[11px] text-muted-foreground">emails, phone numbers, ID & bank cards</div>
            </div>
            <Switch
              checked={config.personal}
              onCheckedChange={(v) => set("personal", v)}
            />
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div className="flex items-center justify-between rounded-sm border border-border px-3 py-2.5">
            <div>
              <div className="text-[13px]">words: case-insensitive</div>
              <div className="text-[11px] text-muted-foreground">mask custom words in any case</div>
            </div>
            <Switch
              checked={config.wordsAnyCase ?? false}
              onCheckedChange={(v) => set("wordsAnyCase", v)}
            />
          </div>
          <div className="flex items-center justify-between rounded-sm border border-border px-3 py-2.5">
            <div>
              <div className="text-[13px]">words: whole word only</div>
              <div className="text-[11px] text-muted-foreground">ABC masks &quot;ABC is&quot; but not XABCD</div>
            </div>
            <Switch
              checked={config.wordsWhole ?? false}
              onCheckedChange={(v) => set("wordsWhole", v)}
            />
          </div>
        </div>

        {/* Custom words */}
        <div className="space-y-2">
          <Label>custom words ({(config.words ?? []).length})</Label>
          <div className="flex flex-wrap gap-1.5">
            {(config.words ?? []).map((w, i) => (
              <Badge key={`${w}-${i}`} variant="outline" className="gap-1 font-mono">
                {w}
                <button
                  className="text-muted-foreground hover:text-destructive"
                  onClick={() => set("words", (config.words ?? []).filter((_, j) => j !== i))}
                >
                  ×
                </button>
              </Badge>
            ))}
          </div>
          <div className="flex gap-2">
            <Input
              value={newWord}
              onChange={(e) => setNewWord(e.target.value)}
              placeholder="word to always mask"
              className="font-mono text-[12px]"
              onKeyDown={(e) => e.key === "Enter" && addWord()}
            />
            <Button variant="outline" size="sm" onClick={addWord}>
              <Plus className="h-3.5 w-3.5" />
              add
            </Button>
          </div>
        </div>

        {/* Custom rules */}
        <div className="space-y-2">
          <Label>custom rules ({(config.rules ?? []).length}/32)</Label>
          <div className="space-y-1.5">
            {(config.rules ?? []).map((r, i) => (
              <div key={i} className="flex items-center gap-2 rounded-sm border border-border/60 px-2.5 py-1.5">
                <Badge variant="accent" className="font-mono text-[10px]">{r.kind}</Badge>
                <span className="truncate font-mono text-[11px] text-muted-foreground">
                  {r.prefix ? `prefix: ${r.prefix}` : `regex: ${r.regex}`}
                </span>
                <button
                  className="ml-auto text-muted-foreground hover:text-destructive"
                  onClick={() => set("rules", (config.rules ?? []).filter((_, j) => j !== i))}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
          </div>
          <div className="grid grid-cols-1 gap-2 md:grid-cols-4">
            <Input
              value={newRuleKind}
              onChange={(e) => setNewRuleKind(e.target.value)}
              placeholder="KIND (default CUSTOM)"
              className="font-mono text-[12px]"
            />
            <Input
              value={newRulePrefix}
              onChange={(e) => setNewRulePrefix(e.target.value)}
              placeholder="literal prefix, e.g. oc_sk_"
              className="font-mono text-[12px]"
            />
            <Input
              value={newRuleRegex}
              onChange={(e) => setNewRuleRegex(e.target.value)}
              placeholder="regex (group 1 = value)"
              className="font-mono text-[12px]"
            />
            <Button variant="outline" size="sm" onClick={addRule}>
              <Plus className="h-3.5 w-3.5" />
              add rule
            </Button>
          </div>
        </div>

        {/* Preview / try it */}
        <div className="space-y-2">
          <Label>try it</Label>
          <textarea
            value={previewIn}
            onChange={(e) => setPreviewIn(e.target.value)}
            placeholder="paste a sample prompt with a key like sk-abc123def456 or an email"
            className="min-h-[64px] w-full rounded-sm border border-border bg-background/50 p-2 font-mono text-[12px] text-foreground focus-visible:outline-none focus-visible:border-primary/60"
          />
          <Button variant="outline" size="sm" onClick={runPreview} disabled={!previewIn.trim()}>
            <Wand2 className="h-3.5 w-3.5" />
            preview
          </Button>
          {previewOut && (
            <pre className="max-h-40 overflow-auto rounded-sm bg-background/60 p-3 font-mono text-[11px] text-muted-foreground">
              {previewOut}
            </pre>
          )}
        </div>
      </PanelBody>
    </Panel>
  );
}
