const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const uploadsRoot = path.join(__dirname, '..', 'uploads', 'products');

const ensureUploadsDir = async () => {
  await fs.promises.mkdir(uploadsRoot, { recursive: true });
};

const buildFileName = image => {
  const safeOriginalName = (image.originalname || 'product-image')
    .replace(/[^a-zA-Z0-9._-]/g, '-')
    .toLowerCase();
  const timestamp = Date.now();
  const randomSuffix = crypto.randomBytes(6).toString('hex');

  return `${timestamp}-${randomSuffix}-${safeOriginalName}`;
};

exports.localUpload = async (image, baseUrl) => {
  try {
    let imageUrl = '';
    let imageKey = '';

    if (image) {
      await ensureUploadsDir();
      const fileName = buildFileName(image);
      const filePath = path.join(uploadsRoot, fileName);
      await fs.promises.writeFile(filePath, image.buffer);

      imageKey = fileName;
      imageUrl = `${baseUrl}/uploads/products/${fileName}`;
    }

    return { imageUrl, imageKey };
  } catch (error) {
    return { imageUrl: '', imageKey: '' };
  }
};
