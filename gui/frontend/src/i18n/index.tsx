import { createContext, useContext, useMemo, type ReactNode } from "react";
import { en, type Key, type Plural } from "./en";
import { es } from "./es";

export type Lang = "es" | "en";
export type Vars = Record<string, string | number>;
export type Translate = (key: Key, vars?: Vars) => string;

const catalogs = { en, es } as const;
const locales: Record<Lang, string> = { en: "en-US", es: "es-MX" };

function isPlural(m: string | Plural): m is Plural {
  return typeof m === "object";
}

/** translator builds t() for one language: plurals pick by vars.count. */
export function translator(lang: Lang): Translate {
  const catalog = catalogs[lang];
  const plurals = new Intl.PluralRules(locales[lang]);
  const numbers = new Intl.NumberFormat(locales[lang]);
  return (key, vars = {}) => {
    const message = catalog[key];
    const text = isPlural(message)
      ? plurals.select(Number(vars.count ?? 0)) === "one"
        ? message.one
        : message.other
      : message;
    return text.replace(/\{(\w+)\}/g, (whole, name: string) => {
      const value = vars[name];
      if (value === undefined) return whole;
      return typeof value === "number" ? numbers.format(value) : value;
    });
  };
}

const I18n = createContext<{ lang: Lang; t: Translate }>({ lang: "es", t: translator("es") });

export function I18nProvider({ lang, children }: { lang: Lang; children: ReactNode }) {
  const value = useMemo(() => ({ lang, t: translator(lang) }), [lang]);
  return <I18n.Provider value={value}>{children}</I18n.Provider>;
}

export function useT() {
  return useContext(I18n);
}

export function asLang(value: string | undefined): Lang {
  return value === "en" ? "en" : "es";
}
