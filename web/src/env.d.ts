import "react";

declare module "react" {
  namespace JSX {
    interface IntrinsicElements {
      // Web component from @hyperframes/player
      "hyperframes-player": React.DetailedHTMLProps<React.HTMLAttributes<HTMLElement>, HTMLElement> & { src?: string; type?: string; controls?: boolean; muted?: boolean; poster?: string };
    }
  }
}
