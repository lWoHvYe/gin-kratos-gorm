import {
  Button,
  Card,
  Cell,
  CellGroup,
  Empty,
  Field,
  Form,
  NavBar,
  Search,
  Tabbar,
  TabbarItem,
  Tag,
} from 'vant'

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.vueApp
    .use(Button)
    .use(Card)
    .use(Cell)
    .use(CellGroup)
    .use(Empty)
    .use(Field)
    .use(Form)
    .use(NavBar)
    .use(Search)
    .use(Tabbar)
    .use(TabbarItem)
    .use(Tag)
})
