# Image credits

The four input images replace the original repository's samples. All are
public domain or CC0, so they can be redistributed freely. They were taken
from the sample data shipped with [scikit-image](https://scikit-image.org/)
(`skimage.data`), whose documentation records each source and license.

| File | Size (pixels) | Subject | Source and license |
|---|---|---|---|
| `retina.jpg` | 1411 × 1411 | Fundus photograph of a normal left eye | Mikael Häggström, Wikimedia Commons; CC0 1.0 |
| `hubble_deep_field.jpg` | 1000 × 872 | Hubble eXtreme Deep Field | NASA / HubbleSite; public domain |
| `spacex_rocket.jpg` | 640 × 427 | SpaceX rocket launch | SpaceX Photos on Flickr; released into the public domain |
| `coffee.png` | 600 × 400 | Coffee cup | Rachel Michetti, courtesy of Pikolo Espresso Bar; CC0 |

They differ in size, shape (square, near-square, and 3:2 landscape), and
format (three JPEG, one PNG), which exercises the pipeline's decoding and
aspect-ratio handling.
