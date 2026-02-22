import { Link } from "react-router";
import { ExternalLink } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ROUTES } from "@/lib/constants";

/* eslint-disable @typescript-eslint/no-explicit-any */
interface Props {
  data: Record<string, any> | undefined;
}

export function TtsSection({ data }: Props) {
  if (!data) return null;

  const provider = data.provider as string | undefined;
  const auto = data.auto as string | undefined;

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center gap-2">
          <CardTitle className="text-base">Text-to-Speech</CardTitle>
          <Badge variant={provider ? "default" : "secondary"}>
            {provider ? "Configured" : "Disabled"}
          </Badge>
        </div>
        <CardDescription>
          {provider
            ? `Provider: ${provider}, Auto: ${auto ?? "off"}`
            : "No TTS provider configured"}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Link
          to={ROUTES.TTS}
          className="inline-flex items-center gap-1.5 text-sm text-primary hover:underline"
        >
          Manage TTS settings <ExternalLink className="h-3.5 w-3.5" />
        </Link>
      </CardContent>
    </Card>
  );
}
