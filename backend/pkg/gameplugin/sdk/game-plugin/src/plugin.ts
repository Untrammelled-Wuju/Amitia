import { Descriptor } from './descriptor';

export interface Plugin {
  descriptor(): Descriptor;
}
