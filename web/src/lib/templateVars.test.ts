import { describe, expect, it } from "vitest";
import { UNSUBSCRIBE_TOKEN, buildFallbackToken, parseToken, upgradeVariableTokens } from "./templateVars";
import { linkifyUnsubscribe, normalizeBareTokens, renderPreview } from "@/components/app/campaigns/sequences/emailPreview";

describe("buildFallbackToken / parseToken", () => {
    it("emits the or form and escapes quotes", () => {
        expect(buildFallbackToken("FirstName", "there")).toBe('{{or .FirstName "there"}}');
        expect(buildFallbackToken("FirstName", "")).toBe("{{.FirstName}}");
        expect(buildFallbackToken("FirstName", "  ")).toBe("{{.FirstName}}");
        expect(buildFallbackToken("Company", 'say "hi"')).toBe('{{or .Company "say \\"hi\\""}}');
    });

    it("parses both fallback spellings back to key and fallback", () => {
        expect(parseToken('{{or .FirstName "there"}}')).toEqual({ key: "FirstName", fallback: "there" });
        expect(parseToken('{{or .Company "say \\"hi\\""}}')).toEqual({ key: "Company", fallback: 'say "hi"' });
        expect(parseToken('{{.FirstName | default "there"}}')).toEqual({ key: "FirstName", fallback: "there" });
        expect(parseToken("{{.FirstName}}")).toEqual({ key: "FirstName", fallback: null });
        expect(parseToken("{{if .FirstName}}")).toBeNull();
    });

    it("chips an or-fallback token on load", () => {
        expect(upgradeVariableTokens('<p>Hi {{or .FirstName "there"}}</p>')).toBe(
            '<p>Hi <span data-var="">{{or .FirstName "there"}}</span></p>',
        );
    });
});

describe("renderPreview", () => {
    it("resolves fallbacks in both spellings", () => {
        const ctx = { FirstName: "", Company: "Acme" };
        expect(renderPreview('Hi {{or .FirstName "there"}} at {{or .Company "your company"}}', ctx)).toBe("Hi there at Acme");
        expect(renderPreview('Hi {{.FirstName | default "friend"}}', ctx)).toBe("Hi friend");
        expect(renderPreview('{{or .FirstName "say \\"hi\\""}}', ctx)).toBe('say "hi"');
    });

    it("renders the sender fields from the sample", () => {
        expect(renderPreview("{{.SenderFirstName}} at {{.SenderCompany}}")).toBe("Sam at Your Company");
    });

    it("reads bare tokens as fields", () => {
        expect(normalizeBareTokens("Hi {{firstName}}, {{ FirstName }}, {{first_name}}")).toBe(
            "Hi {{.FirstName}}, {{.FirstName}}, {{.FirstName}}",
        );
        expect(normalizeBareTokens("{{senderFirstName}} {{sender_full_name}}")).toBe("{{.SenderFirstName}} {{.SenderName}}");
        expect(normalizeBareTokens("{{role}}")).toBe("{{.role}}");
        expect(normalizeBareTokens("{{if .X}}{{end}}{{else}}")).toBe("{{if .X}}{{end}}{{else}}");
        expect(renderPreview("Hi {{firstName}} from {{senderCompany}}")).toBe("Hi Alex from Your Company");
        expect(renderPreview("{{if .Company}}{{company}}{{end}}", { Company: "Acme" })).toBe("Acme");
    });
});

describe("upgradeVariableTokens", () => {
    it("chips a token in text", () => {
        expect(upgradeVariableTokens("<p>Hi {{.FirstName}}</p>")).toBe('<p>Hi <span data-var="">{{.FirstName}}</span></p>');
    });

    it("leaves a token that is an attribute value alone", () => {
        // A link the author pointed at the unsubscribe variable: wrapping the
        // token in a span here would break the tag.
        const html = `<p><a href="${UNSUBSCRIBE_TOKEN}">no thanks</a></p>`;
        expect(upgradeVariableTokens(html)).toBe(html);
    });

    it("keeps a tag whose attribute contains a >", () => {
        // Reading the quoted ">" as the end of the tag split it, and the href
        // that followed was then wrapped in a chip span.
        const html = `<p><a title="x > y" href="${UNSUBSCRIBE_TOKEN}">no thanks</a></p>`;
        expect(upgradeVariableTokens(html)).toBe(html);
        expect(upgradeVariableTokens(`<p title="a > b">Hi {{.FirstName}}</p>`)).toBe(
            '<p title="a > b">Hi <span data-var="">{{.FirstName}}</span></p>',
        );
    });

    it("is a no-op once the content already carries chips", () => {
        const html = '<p><span data-var="">{{.FirstName}}</span> {{.Company}}</p>';
        expect(upgradeVariableTokens(html)).toBe(html);
    });
});

describe("linkifyUnsubscribe", () => {
    const url = "https://example.com/unsubscribe/preview";

    it("wraps a loose unsubscribe URL in an anchor", () => {
        expect(linkifyUnsubscribe(`<p>Bye. ${url}</p>`)).toBe(`<p>Bye. <a href="${url}">Unsubscribe</a></p>`);
    });

    it("leaves an author's own anchor alone", () => {
        const html = `<p><a href="${url}">no thanks</a></p>`;
        expect(linkifyUnsubscribe(html)).toBe(html);
    });

    it("labels the URL when it is an anchor's own text", () => {
        expect(linkifyUnsubscribe(`<a href="${url}">${url}</a>`)).toBe(`<a href="${url}">Unsubscribe</a>`);
    });

    it("does not rewrite an href behind a quoted > in the same tag", () => {
        const html = `<a title="x > y" href="${url}">read this</a>`;
        expect(linkifyUnsubscribe(html)).toBe(html);
    });

    it("does nothing when the body has no link", () => {
        expect(linkifyUnsubscribe("<p>Hi</p>")).toBe("<p>Hi</p>");
    });
});
