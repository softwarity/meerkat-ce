// The shell's own words. Everything a reader actually reads lives in Markdown
// under content/<lang>; what is left is this - labels on chrome that has no
// prose. Twenty-odd strings, one object, ONE bundle serving every language.
//
// That is the reason there is no @angular/localize here: the site would then
// have to be compiled once per language and deployed as several apps, to
// translate a menu. Adding a language is adding a folder under content/ and an
// entry below.

export const LANGS = ['en', 'fr'] as const;
export type Lang = (typeof LANGS)[number];

export const LANG_NAMES: Record<Lang, string> = {
  en: 'English',
  fr: 'Français',
};

interface Words {
  search: string;
  searchHint: string;
  noResult: string;
  onThisPage: string;
  language: string;
  appearance: string;
  system: string;
  light: string;
  dark: string;
  version: string;
  versionNext: string;
  readingOld: string;
  readingNext: string;
  goCurrent: string;
  notFound: string;
  backHome: string;
  madeBy: string;
  menu: string;
  close: string;
  inSection: string;
  print: string;
  previousImage: string;
  nextImage: string;
  imageOf: string;
}

export const WORDS: Record<Lang, Words> = {
  en: {
    search: 'Search',
    searchHint: 'Search the site',
    noResult: 'Nothing found',
    onThisPage: 'On this page',
    language: 'Language',
    appearance: 'Appearance',
    system: 'System',
    light: 'Light',
    dark: 'Dark',
    version: 'Version',
    versionNext: 'next (unreleased)',
    readingOld: 'You are reading the documentation for',
    readingNext: 'You are reading the documentation for the next release, which is not out yet.',
    goCurrent: 'Read the current version',
    notFound: 'There is no page here.',
    backHome: 'Back to the home page',
    madeBy: 'Made by',
    menu: 'Menu',
    close: 'Close',
    inSection: 'in',
    print: 'Print',
    previousImage: 'Previous image',
    nextImage: 'Next image',
    imageOf: 'of',
  },
  fr: {
    search: 'Rechercher',
    searchHint: 'Rechercher dans le site',
    noResult: 'Aucun résultat',
    onThisPage: 'Sur cette page',
    language: 'Langue',
    appearance: 'Apparence',
    system: 'Système',
    light: 'Clair',
    dark: 'Sombre',
    version: 'Version',
    versionNext: 'next (non publiée)',
    readingOld: 'Vous lisez la documentation de la version',
    readingNext: "Vous lisez la documentation de la prochaine version, qui n'est pas encore publiée.",
    goCurrent: 'Lire la version courante',
    notFound: "Il n'y a pas de page ici.",
    backHome: "Revenir à l'accueil",
    madeBy: 'Réalisé par',
    menu: 'Menu',
    close: 'Fermer',
    inSection: 'dans',
    print: 'Imprimer',
    previousImage: 'Image précédente',
    nextImage: 'Image suivante',
    imageOf: 'sur',
  },
};
