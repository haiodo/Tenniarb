import Vue from "vue";
import VueRouter from "vue-router";

import FeaturesPage from "../views/Features.vue";
import BlogPage from "../views/Blog.vue";
import DocsPage from "../views/Docs.vue";

Vue.use(VueRouter);

const routes = [
  {
    path: "/",
    name: "Features",
    component: FeaturesPage
  },
  {
    path: "/blog",
    name: "Blog",
    component: BlogPage
  },
  {
    path: "/docs",
    name: "Documentation",
    component: DocsPage
  },
  {
    path: "/about",
    name: "About",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () =>
      import(/* webpackChunkName: "about" */ "../views/About.vue")
  }
];

const router = new VueRouter({
  mode: "history",
  base: process.env.BASE_URL,
  routes
});

export default router;
