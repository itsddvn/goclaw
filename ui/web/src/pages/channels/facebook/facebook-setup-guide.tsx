import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, CircleAlert, Copy, ExternalLink, MessageCircle, RefreshCw } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useClipboard } from "@/hooks/use-clipboard";
import {
  facebookCommentScopes,
  facebookMessengerScopes,
  facebookUseCaseScopes,
} from "../channel-schemas";
import {
  generateVerifyToken,
  getFacebookCallbackUrl,
  isPublicHttpsOrigin,
} from "./facebook-setup";

const META_LINKS = [
  {
    href: "https://developers.facebook.com/documentation/development/create-an-app/messenger-use-case",
    key: "useCase",
  },
  {
    href: "https://developers.facebook.com/documentation/business-messaging/messenger-platform/get-started",
    key: "getStarted",
  },
  {
    href: "https://developers.facebook.com/documentation/business-messaging/messenger-platform/webhooks",
    key: "webhooks",
  },
] as const;

interface FacebookSetupGuideProps {
  verifyToken: string;
  onVerifyTokenChange: (value: string) => void;
}

export function FacebookSetupGuide({
  verifyToken,
  onVerifyTokenChange,
}: FacebookSetupGuideProps) {
  const { t } = useTranslation("channels");
  const callbackClipboard = useClipboard();
  const tokenClipboard = useClipboard();
  const [tokenGenerationFailed, setTokenGenerationFailed] = useState(false);
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  const callbackUrl = getFacebookCallbackUrl(origin);
  const callbackReady = isPublicHttpsOrigin(origin);

  const handleGenerateToken = () => {
    try {
      onVerifyTokenChange(generateVerifyToken());
      setTokenGenerationFailed(false);
    } catch {
      setTokenGenerationFailed(true);
    }
  };

  const handleCopyCallback = () => {
    if (callbackReady) void callbackClipboard.copy(callbackUrl);
  };

  const handleCopyToken = () => {
    if (verifyToken) void tokenClipboard.copy(verifyToken);
  };

  return (
    <section
      aria-labelledby="facebook-setup-title"
      className="rounded-md border bg-muted/30 p-3 text-sm"
    >
      <header className="flex items-start gap-2">
        <MessageCircle className="mt-0.5 size-4 shrink-0 text-primary" aria-hidden="true" />
        <div className="min-w-0 space-y-1">
          <h3 id="facebook-setup-title" className="font-medium">
            {t("facebook.setupGuide.title")}
          </h3>
          <p className="text-xs leading-relaxed text-muted-foreground">
            {t("facebook.setupGuide.intro")}
          </p>
        </div>
      </header>

      <ol className="mt-3 list-decimal space-y-2 pl-5 text-xs leading-relaxed text-muted-foreground marker:font-medium marker:text-foreground">
        <li>
          <span className="font-medium text-foreground">{t("facebook.setupGuide.steps.connectTitle")}</span>{" "}
          {t("facebook.setupGuide.steps.connectDescription")}
          <div className="mt-1 grid gap-1 border-l pl-2">
            <p>
              <span className="font-medium text-foreground">{t("facebook.setupGuide.paths.currentLabel")}:</span>{" "}
              {t("facebook.setupGuide.paths.current")}
            </p>
            <p>
              <span className="font-medium text-foreground">{t("facebook.setupGuide.paths.legacyLabel")}:</span>{" "}
              {t("facebook.setupGuide.paths.legacy")}
            </p>
          </div>
        </li>
        <li>
          <span className="font-medium text-foreground">{t("facebook.setupGuide.steps.secretTitle")}</span>{" "}
          {t("facebook.setupGuide.steps.secretDescription")}
        </li>
        <li>
          <span className="font-medium text-foreground">{t("facebook.setupGuide.steps.verifyTitle")}</span>{" "}
          {t("facebook.setupGuide.steps.verifyDescription")}
        </li>
        <li>
          <span className="font-medium text-foreground">{t("facebook.setupGuide.steps.saveTitle")}</span>{" "}
          {t("facebook.setupGuide.steps.saveDescription")}
        </li>
        <li>
          <span className="font-medium text-foreground">{t("facebook.setupGuide.steps.webhookTitle")}</span>{" "}
          {t("facebook.setupGuide.steps.webhookDescription")}{" "}
          <code className="text-foreground">messages</code>,{" "}
          <code className="text-foreground">message_echoes</code>,{" "}
          <code className="text-foreground">messaging_postbacks</code>;{" "}
          {t("facebook.setupGuide.steps.webhookComments")}{" "}
          <code className="text-foreground">feed</code>.
        </li>
      </ol>

      <div className="mt-3 space-y-3 border-t pt-3">
        <div className="space-y-1.5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="text-xs font-medium">{t("facebook.setupGuide.callback.label")}</span>
            <Badge variant={callbackReady ? "success" : "warning"}>
              {callbackReady
                ? t("facebook.setupGuide.callback.ready")
                : t("facebook.setupGuide.callback.notReady")}
            </Badge>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row">
            <code className="min-w-0 flex-1 break-all rounded-md border bg-background px-3 py-2 text-xs text-foreground">
              {callbackUrl}
            </code>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleCopyCallback}
              disabled={!callbackReady}
            >
              {callbackClipboard.copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
              {callbackClipboard.copied
                ? t("facebook.setupGuide.actions.copied")
                : t("facebook.setupGuide.actions.copyCallback")}
            </Button>
          </div>
        </div>

        {!callbackReady && (
          <Alert className="bg-background">
            <CircleAlert aria-hidden="true" />
            <AlertTitle>{t("facebook.setupGuide.callback.warningTitle")}</AlertTitle>
            <AlertDescription>
              <p>{t("facebook.setupGuide.callback.warningDescription")}</p>
            </AlertDescription>
          </Alert>
        )}

        <div className="space-y-1.5">
          <div>
            <p className="text-xs font-medium">{t("facebook.setupGuide.verifyToken.label")}</p>
            <p className="text-xs leading-relaxed text-muted-foreground">
              {t("facebook.setupGuide.verifyToken.description")}
            </p>
          </div>
          <Input
            value={verifyToken}
            readOnly
            autoComplete="off"
            className="font-mono"
            aria-label={t("facebook.setupGuide.verifyToken.label")}
            placeholder={t("facebook.setupGuide.verifyToken.placeholder")}
          />
          <div className="flex flex-wrap gap-2">
            <Button type="button" variant="secondary" size="sm" onClick={handleGenerateToken}>
              <RefreshCw aria-hidden="true" />
              {verifyToken
                ? t("facebook.setupGuide.actions.regenerateToken")
                : t("facebook.setupGuide.actions.generateToken")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleCopyToken}
              disabled={!verifyToken}
            >
              {tokenClipboard.copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
              {tokenClipboard.copied
                ? t("facebook.setupGuide.actions.copied")
                : t("facebook.setupGuide.actions.copyToken")}
            </Button>
          </div>
          {tokenGenerationFailed && (
            <p role="alert" className="text-xs text-destructive">
              {t("facebook.setupGuide.verifyToken.error")}
            </p>
          )}
        </div>
      </div>

      <div className="mt-3 space-y-2 border-t pt-3 text-xs leading-relaxed text-muted-foreground">
        <p>
          <span className="font-medium text-foreground">{t("facebook.setupGuide.permissions.messengerTitle")}:</span>{" "}
          {facebookMessengerScopes.map((scope, index) => (
            <span key={scope}>
              {index > 0 && ", "}
              <code className="text-foreground">{scope}</code>
            </span>
          ))}
          . {t("facebook.setupGuide.permissions.useCasePrefix")}{" "}
          {facebookUseCaseScopes.map((scope, index) => (
            <span key={scope}>
              {index > 0 && ", "}
              <code className="text-foreground">{scope}</code>
            </span>
          ))}
          .
        </p>
        <p>
          <span className="font-medium text-foreground">{t("facebook.setupGuide.permissions.commentsTitle")}:</span>{" "}
          {facebookCommentScopes.map((scope, index) => (
            <span key={scope}>
              {index > 0 && ", "}
              <code className="text-foreground">{scope}</code>
            </span>
          ))}
          . {t("facebook.setupGuide.permissions.commentsDescription")}
        </p>
        <p>{t("facebook.setupGuide.permissions.accessReview")}</p>
        <p className="font-medium text-foreground">{t("facebook.setupGuide.messengerRequired")}</p>
      </div>

      <nav aria-label={t("facebook.setupGuide.links.label")} className="mt-3 flex flex-wrap gap-x-3 gap-y-1 border-t pt-3">
        {META_LINKS.map((link) => (
          <a
            key={link.key}
            href={link.href}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-xs font-medium text-primary underline-offset-4 hover:underline focus-visible:rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {t(`facebook.setupGuide.links.${link.key}`)}
            <ExternalLink className="size-3" aria-hidden="true" />
          </a>
        ))}
      </nav>
    </section>
  );
}
