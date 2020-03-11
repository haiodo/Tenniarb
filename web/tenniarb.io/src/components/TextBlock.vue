<script lang="ts">
import Vue, { VueConstructor } from "vue";
import Component from "vue-class-component";
import { Prop } from "vue-property-decorator";

@Component({
  components: {}
})
export default class TextBlock extends Vue {
  @Prop({ required: true, type: String })
  title!: string;

  @Prop({ required: true, type: String })
  subtitle!: string;
}
</script>

<template>
  <div class="text-block">
    <div class="td title">
      <div>{{title}}</div>
      <div class="hr" />
      <div class="subtitle">{{subtitle}}</div>
    </div>
    <div class="td right">
      <slot />
    </div>
  </div>
</template>

<style scoped lang="scss">
.text-block {
  background-color: #2c3e50;
  padding: 2em;
  display: flex;
  flex-flow: row;
  flex-wrap: wrap;
  justify-content: center;
  .title {
    text-align: right;
    font-size: 3rem;
    font-weight: 700 !important;
    color: white;
    flex-grow: 1;
    max-width: 10em;
    .subtitle {
      font-size: 1.5rem;
    }
    .hr {
      font-size: 0.5em;
      background-color: #f34b13;
      height: 0.1em;
    }
  }
  .right {
    text-align: left;
    padding-left: 2em;
  }
}
</style>
